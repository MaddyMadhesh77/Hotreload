//go:build windows

package builder

import (
	"os/exec"
	"syscall"
)

// setProcGroup on Windows uses CREATE_NEW_PROCESS_GROUP so that the child
// can receive Ctrl-Break signals and be terminated cleanly.
func setProcGroup(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{
		CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP,
	}
}
