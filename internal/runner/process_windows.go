//go:build windows

package runner

import (
	"log/slog"
	"os/exec"
	"syscall"
	"time"
	"unsafe"
)

var (
	modkernel32                  = syscall.NewLazyDLL("kernel32.dll")
	procAssignProcessToJobObject = modkernel32.NewProc("AssignProcessToJobObject")
	procCreateJobObjectW         = modkernel32.NewProc("CreateJobObjectW")
	procSetInformationJobObject  = modkernel32.NewProc("SetInformationJobObject")
	procTerminateJobObject       = modkernel32.NewProc("TerminateJobObject")
)

const (
	jobObjectInfoLimitInformation = 2
	jobObjectLimitKillOnJobClose  = 0x2000
)

type jobobjectBasicLimitInformation struct {
	PerProcessUserTimeLimit int64
	PerJobUserTimeLimit     int64
	LimitFlags              uint32
	MinimumWorkingSetSize   uintptr
	MaximumWorkingSetSize   uintptr
	ActiveProcessLimit      uint32
	Affinity                uintptr
	PriorityClass           uint32
	SchedulingClass         uint32
}

type ioCounters struct {
	ReadOperationCount  uint64
	WriteOperationCount uint64
	OtherOperationCount uint64
	ReadTransferCount   uint64
	WriteTransferCount  uint64
	OtherTransferCount  uint64
}

type jobobjectExtendedLimitInformation struct {
	BasicLimitInformation jobobjectBasicLimitInformation
	IoInfo                ioCounters
	ProcessMemoryLimit    uintptr
	JobMemoryLimit        uintptr
	PeakProcessMemoryUsed uintptr
	PeakJobMemoryUsed     uintptr
}

// jobHandle is a Windows Job Object that owns the server process tree.
// When we close the handle the OS terminates all processes in the job.
var jobHandle syscall.Handle

func init() {
	// Create a Job Object on startup. All server processes are assigned to it.
	h, _, _ := procCreateJobObjectW.Call(0, 0)
	if h == 0 {
		return
	}
	jobHandle = syscall.Handle(h)

	// Set KILL_ON_JOB_CLOSE so the OS terminates children when we close it.
	info := jobobjectExtendedLimitInformation{}
	info.BasicLimitInformation.LimitFlags = jobObjectLimitKillOnJobClose
	procSetInformationJobObject.Call(
		uintptr(jobHandle),
		jobObjectInfoLimitInformation,
		uintptr(unsafe.Pointer(&info)),
		uintptr(unsafe.Sizeof(info)),
	)
}

// setProcGroup on Windows creates a new process group so Ctrl-Break can target it.
func setProcGroup(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{
		CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP,
	}
}

// killProcess on Windows assigns the process to a Job Object so all children
// are tracked, sends Ctrl-Break for graceful shutdown, then terminates the job.
func killProcess(cmd *exec.Cmd, timeout time.Duration) {
	if cmd.Process == nil {
		return
	}

	pid := cmd.Process.Pid

	const processAllAccess = 0x1F0FFF
	// Assign to job so TerminateJobObject kills the whole tree.
	if jobHandle != 0 {
		handle, err := syscall.OpenProcess(processAllAccess, false, uint32(pid))
		if err == nil {
			procAssignProcessToJobObject.Call(uintptr(jobHandle), uintptr(handle))
			_ = syscall.CloseHandle(handle)
		}
	}

	// Send Ctrl-Break for graceful shutdown.
	_ = cmd.Process.Signal(syscall.Signal(0x15)) // SIGBREAK on Windows

	done := make(chan struct{})
	go func() {
		_ = cmd.Wait()
		close(done)
	}()

	select {
	case <-done:
		return
	case <-time.After(timeout):
		slog.Warn("runner: process did not exit after Ctrl-Break, terminating job",
			"pid", pid)
		if jobHandle != 0 {
			procTerminateJobObject.Call(uintptr(jobHandle), 1)
		} else {
			_ = cmd.Process.Kill()
		}
		<-done
	}
}
