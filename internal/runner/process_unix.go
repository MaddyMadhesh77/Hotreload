//go:build !windows

package runner

import (
	"log/slog"
	"os/exec"
	"syscall"
	"time"
)

// setProcGroup puts the child into its own process group on Unix so we can
// send a signal to the entire group (parent + all spawned children).
func setProcGroup(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

// killProcess attempts a graceful shutdown of cmd via SIGTERM, then forcefully
// kills the whole process group with SIGKILL if the process doesn't exit within
// the given timeout.
func killProcess(cmd *exec.Cmd, timeout time.Duration) {
	if cmd.Process == nil {
		return
	}

	pid := cmd.Process.Pid

	// Send SIGTERM to the whole process group (negative pid).
	pgid, err := syscall.Getpgid(pid)
	if err == nil {
		_ = syscall.Kill(-pgid, syscall.SIGTERM)
	} else {
		_ = cmd.Process.Signal(syscall.SIGTERM)
	}

	// Wait for the process with a deadline.
	done := make(chan struct{})
	go func() {
		_ = cmd.Wait()
		close(done)
	}()

	select {
	case <-done:
		return // exited cleanly
	case <-time.After(timeout):
		slog.Warn("runner: process did not exit after SIGTERM, sending SIGKILL",
			"pid", pid, "timeout", timeout)
		// Kill the entire process group.
		if err == nil {
			_ = syscall.Kill(-pgid, syscall.SIGKILL)
		} else {
			_ = cmd.Process.Kill()
		}
		<-done
	}
}
