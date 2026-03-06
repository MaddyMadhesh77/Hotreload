// Package runner manages the lifecycle of the user's server process.
// It is responsible for:
//
//   - Starting the server and streaming its logs in real time.
//   - Stopping the server gracefully (SIGTERM → wait → SIGKILL).
//   - Killing the entire process group so orphaned children don't linger.
//   - Detecting crash loops and applying a cool-down to avoid spinning.
package runner

import (
	"log/slog"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"
)

const (
	// gracefulTimeout is how long we wait after SIGTERM before sending SIGKILL.
	gracefulTimeout = 3 * time.Second

	// crashThreshold: if the process exits within crashWindow after starting
	// more than crashThreshold times in a row, we call it a crash loop.
	crashThreshold = 3
	crashWindow    = 2 * time.Second
	crashCooldown  = 5 * time.Second
)

// Runner manages a single server process at a time.
type Runner struct {
	command string

	mu         sync.Mutex
	cmd        *exec.Cmd
	stopCh     chan struct{} // closed to signal the current process reaper
	crashTimes []time.Time   // timestamps of recent fast crashes
}

// New creates a Runner for the given shell command string.
func New(command string) *Runner {
	return &Runner{command: command}
}

// Start launches the server process. If a process is already running it is
// stopped first. This call is non-blocking: the process runs in the background.
func (r *Runner) Start() {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Apply crash-loop cooldown before starting.
	if r.isCrashLooping() {
		slog.Warn("runner: crash loop detected, waiting before restart",
			"cooldown", crashCooldown)
		time.Sleep(crashCooldown)
		// Reset crash history after cooldown.
		r.crashTimes = nil
	}

	parts := splitCommand(r.command)
	if len(parts) == 0 {
		slog.Error("runner: empty exec command")
		return
	}

	cmd := exec.Command(parts[0], parts[1:]...)
	cmd.Env = os.Environ()
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	setProcGroup(cmd)

	if err := cmd.Start(); err != nil {
		slog.Error("runner: failed to start server", "err", err)
		return
	}

	slog.Info("runner: server started", "pid", cmd.Process.Pid, "cmd", r.command)

	r.cmd = cmd
	stopCh := make(chan struct{})
	r.stopCh = stopCh

	startTime := time.Now()

	// Reaper goroutine: waits for the process and detects crashes.
	go func() {
		err := cmd.Wait()
		elapsed := time.Since(startTime)

		select {
		case <-stopCh:
			// We asked it to stop — not a crash.
			slog.Info("runner: server exited after stop request")
		default:
			if err != nil {
				slog.Error("runner: server exited unexpectedly",
					"err", err, "elapsed", elapsed.Round(time.Millisecond))
			} else {
				slog.Warn("runner: server exited cleanly but unexpectedly",
					"elapsed", elapsed.Round(time.Millisecond))
			}

			// Record time for crash-loop detection.
			r.mu.Lock()
			if elapsed < crashWindow {
				r.crashTimes = append(r.crashTimes, time.Now())
			} else {
				r.crashTimes = nil // happy path run reset crash history
			}
			r.mu.Unlock()
		}
	}()
}

// Stop terminates the current server process: SIGTERM first, then SIGKILL
// if the process doesn't exit within gracefulTimeout.
func (r *Runner) Stop() {
	r.mu.Lock()
	cmd := r.cmd
	stopCh := r.stopCh
	r.cmd = nil
	r.stopCh = nil
	r.mu.Unlock()

	if cmd == nil || cmd.Process == nil {
		return
	}

	// Signal the reaper that this is an intentional stop.
	if stopCh != nil {
		close(stopCh)
	}

	slog.Info("runner: stopping server", "pid", cmd.Process.Pid)
	killProcess(cmd, gracefulTimeout)
	slog.Info("runner: server stopped")
}

// isCrashLooping returns true if the process has started and exited quickly
// crashThreshold times in a row. Must be called with r.mu held.
func (r *Runner) isCrashLooping() bool {
	if len(r.crashTimes) < crashThreshold {
		return false
	}
	// Keep only recent entries.
	cutoff := time.Now().Add(-crashWindow * crashThreshold)
	recent := r.crashTimes[:0]
	for _, t := range r.crashTimes {
		if t.After(cutoff) {
			recent = append(recent, t)
		}
	}
	r.crashTimes = recent
	return len(r.crashTimes) >= crashThreshold
}

// splitCommand splits a shell-style command string into argv, handling quotes.
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
	return args
}
