//go:build !windows

package builder

import (
	"os/exec"
	"syscall"
)

// setProcGroup puts the child process into its own process group on Unix/Linux/macOS.
// This ensures that when we kill the process we can also kill all its children
// by sending a signal to the negative of the pgid.
func setProcGroup(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}
