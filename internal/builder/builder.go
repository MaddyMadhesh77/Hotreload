// Package builder runs the user-supplied build command and streams its output
// in real time. It supports cancellation so that a new file-change event can
// abort an in-flight build and start a fresh one — ensuring we always compile
// the latest source state.
package builder

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"strings"
)

// Result holds the outcome of a build attempt.
type Result struct {
	Success bool
	Err     error
}

// Builder executes a build command and streams output to the logger.
type Builder struct {
	// Command is the raw build command string (e.g. "go build -o ./bin/server ./cmd/server").
	Command string
}

// New creates a Builder for the given shell command string.
func New(command string) *Builder {
	return &Builder{Command: command}
}

// Run executes the build command, streaming stdout/stderr in real time.
// It respects ctx: if the context is cancelled the child process is killed.
// Returns true if the command exits with code 0.
func (b *Builder) Run(ctx context.Context) Result {
	slog.Info("builder: starting build", "cmd", b.Command)

	parts := splitCommand(b.Command)
	if len(parts) == 0 {
		return Result{Err: fmt.Errorf("builder: empty command")}
	}

	cmd := exec.CommandContext(ctx, parts[0], parts[1:]...)
	cmd.Env = os.Environ()

	// Stream stdout and stderr to our process output so logs appear in real time.
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	// On supported platforms, set the process group so children are also killed
	// when the context is cancelled. (See process_unix.go / process_windows.go)
	setProcGroup(cmd)

	if err := cmd.Start(); err != nil {
		return Result{Err: fmt.Errorf("builder: failed to start: %w", err)}
	}

	err := cmd.Wait()
	if ctx.Err() != nil {
		// Context was cancelled — treat as a deliberate abort, not an error.
		slog.Info("builder: build cancelled (new change detected)")
		return Result{Success: false, Err: ctx.Err()}
	}

	if err != nil {
		slog.Error("builder: build failed", "err", err)
		return Result{Err: fmt.Errorf("builder: %w", err)}
	}

	slog.Info("builder: build succeeded")
	return Result{Success: true}
}

// splitCommand splits a shell-style command string into argv tokens.
// It handles quoted arguments (single or double quotes) so paths with spaces work.
func splitCommand(s string) []string {
	var args []string
	var current strings.Builder
	inQuote := rune(0)

	flush := func() {
		if current.Len() > 0 {
			args = append(args, current.String())
			current.Reset()
		}
	}

	for _, ch := range s {
		switch {
		case inQuote != 0 && ch == inQuote:
			inQuote = 0
		case inQuote != 0:
			current.WriteRune(ch)
		case ch == '"' || ch == '\'':
			inQuote = ch
		case ch == ' ' || ch == '\t':
			flush()
		default:
			current.WriteRune(ch)
		}
	}
	flush()

	_ = io.Discard // keep import if needed later
	return args
}
