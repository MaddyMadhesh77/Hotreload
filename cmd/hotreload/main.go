// Command hotreload watches a project directory for source-code changes and
// automatically rebuilds and restarts the server on each change.
//
// Usage:
//
//	hotreload --root <dir> --build "<cmd>" --exec "<cmd>"
//
// Example:
//
//	hotreload --root ./myproject \
//	          --build "go build -o ./bin/server ./cmd/server" \
//	          --exec "./bin/server"
package main

import (
	"flag"
	"fmt"
	"log/slog"
	"os"
	osexec "os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"hotreload/internal/engine"
)

func main() {
	root := flag.String("root", ".", "Directory to watch for file changes")
	runTarget := flag.String("run", "", "Go package or directory to build and run with a nodemon-style shorthand")
	build := flag.String("build", "", "Command to build the project (required)")
	exec := flag.String("exec", "", "Command to run the server after a successful build (required)")
	debounce := flag.Duration("debounce", 200*time.Millisecond, "Quiet period after last file event before rebuilding")
	verbose := flag.Bool("v", false, "Enable verbose (debug) logging")

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "hotreload — automatic rebuild & restart on file changes\n\n")
		fmt.Fprintf(os.Stderr, "Usage:\n")
		fmt.Fprintf(os.Stderr, "  hotreload [--root <dir>] --run <go-package> [--debounce <duration>] [-v]\n")
		fmt.Fprintf(os.Stderr, "  hotreload --root <dir> --build \"<cmd>\" --exec \"<cmd>\" [--debounce <duration>] [-v]\n\n")
		fmt.Fprintf(os.Stderr, "Flags:\n")
		flag.PrintDefaults()
	}

	flag.Parse()

	resolvedRoot, resolvedBuild, resolvedExec, cleanup, err := resolveCLIConfig(*root, *runTarget, *build, *exec, flag.Args())
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		flag.Usage()
		os.Exit(1)
	}
	if cleanup != nil {
		defer cleanup()
	}

	// Configure slog: use a text handler for human-readable terminal output.
	logLevel := slog.LevelInfo
	if *verbose {
		logLevel = slog.LevelDebug
	}

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: logLevel,
		// Add source location in verbose mode.
		AddSource: *verbose,
	}))
	slog.SetDefault(logger)

	cfg := engine.Config{
		Root:          resolvedRoot,
		BuildCommand:  resolvedBuild,
		ExecCommand:   resolvedExec,
		DebounceDelay: *debounce,
	}

	if err := engine.Run(cfg); err != nil {
		slog.Error("hotreload: fatal error", "err", err)
		os.Exit(1)
	}
}

func resolveCLIConfig(root, runTarget, build, exec string, args []string) (string, string, string, func(), error) {
	if runTarget != "" && (build != "" || exec != "") {
		return "", "", "", nil, fmt.Errorf("use either --run or --build/--exec, not both")
	}

	if runTarget == "" && len(args) > 0 {
		if args[0] == "go" {
			if len(args) < 3 || args[1] != "run" {
				return "", "", "", nil, fmt.Errorf("expected either one Go target or --build/--exec flags")
			}
			runTarget = args[2]
			args = args[3:]
		} else {
			runTarget = args[0]
			args = args[1:]
		}

		if len(args) > 0 && args[0] == "--" {
			args = args[1:]
		}
	}

	if runTarget == "" && build == "" && exec == "" {
		inferredTarget, err := detectGoTarget(root)
		if err != nil {
			return "", "", "", nil, err
		}
		runTarget = inferredTarget
	}

	if runTarget != "" {
		if root == "" {
			root = "."
		}

		tempDir, err := os.MkdirTemp("", "hotreload-*")
		if err != nil {
			return "", "", "", nil, fmt.Errorf("create temp build dir: %w", err)
		}

		binaryName := "hotreload-server"
		if runtime.GOOS == "windows" {
			binaryName += ".exe"
		}
		binaryPath := filepath.Join(tempDir, binaryName)
		cleanup := func() {
			_ = os.RemoveAll(tempDir)
		}

		build = fmt.Sprintf("go build -o %s %s", quoteShellArg(binaryPath), quoteShellArg(runTarget))
		exec = joinShellArgs(append([]string{binaryPath}, args...)...)
		return root, build, exec, cleanup, nil
	}

	if build == "" {
		return "", "", "", nil, fmt.Errorf("--build is required when --run is not used")
	}
	if exec == "" {
		return "", "", "", nil, fmt.Errorf("--exec is required when --run is not used")
	}

	if strings.TrimSpace(root) == "" {
		root = "."
	}

	return root, build, exec, nil, nil
}

func detectGoTarget(root string) (string, error) {
	if strings.TrimSpace(root) == "" {
		root = "."
	}

	cmd := osexec.Command("go", "list", "-f", "{{if eq .Name \"main\"}}{{.ImportPath}}{{end}}", "./...")
	cmd.Dir = root
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("detect Go target: %w: %s", err, strings.TrimSpace(string(output)))
	}

	var targets []string
	for _, line := range strings.Split(string(output), "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			targets = append(targets, line)
		}
	}

	switch len(targets) {
	case 0:
		return "", fmt.Errorf("could not infer a Go target: no main packages found under %s; pass --run or explicit --build/--exec", root)
	case 1:
		return targets[0], nil
	default:
		return "", fmt.Errorf("could not infer a Go target: found multiple main packages under %s (%s); pass --run explicitly", root, strings.Join(targets, ", "))
	}
}

func quoteShellArg(value string) string {
	return "\"" + value + "\""
}

func joinShellArgs(parts ...string) string {
	quoted := make([]string, 0, len(parts))
	for _, part := range parts {
		quoted = append(quoted, quoteShellArg(part))
	}
	return strings.Join(quoted, " ")
}
