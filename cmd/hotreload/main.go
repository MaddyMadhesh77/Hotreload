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
	"time"

	"hotreload/internal/engine"
)

func main() {
	root := flag.String("root", ".", "Directory to watch for file changes")
	build := flag.String("build", "", "Command to build the project (required)")
	exec := flag.String("exec", "", "Command to run the server after a successful build (required)")
	debounce := flag.Duration("debounce", 200*time.Millisecond, "Quiet period after last file event before rebuilding")
	verbose := flag.Bool("v", false, "Enable verbose (debug) logging")

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "hotreload — automatic rebuild & restart on file changes\n\n")
		fmt.Fprintf(os.Stderr, "Usage:\n")
		fmt.Fprintf(os.Stderr, "  hotreload --root <dir> --build \"<cmd>\" --exec \"<cmd>\" [--debounce <duration>] [-v]\n\n")
		fmt.Fprintf(os.Stderr, "Flags:\n")
		flag.PrintDefaults()
	}

	flag.Parse()

	// Validate required flags.
	if *build == "" {
		fmt.Fprintln(os.Stderr, "error: --build is required")
		flag.Usage()
		os.Exit(1)
	}
	if *exec == "" {
		fmt.Fprintln(os.Stderr, "error: --exec is required")
		flag.Usage()
		os.Exit(1)
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
		Root:          *root,
		BuildCommand:  *build,
		ExecCommand:   *exec,
		DebounceDelay: *debounce,
	}

	if err := engine.Run(cfg); err != nil {
		slog.Error("hotreload: fatal error", "err", err)
		os.Exit(1)
	}
}
