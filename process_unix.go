//go:build !windows

package every

import (
	"os/exec"
	"syscall"
	"time"
)

func configureProcess(cmd *exec.Cmd) { cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true} }

func terminateProcess(cmd *exec.Cmd) {
	if cmd.Process == nil {
		return
	}
	pid := cmd.Process.Pid
	if err := syscall.Kill(-pid, syscall.SIGTERM); err != nil {
		_ = cmd.Process.Kill()
		return
	}
	time.Sleep(300 * time.Millisecond)
	_ = syscall.Kill(-pid, syscall.SIGKILL)
}

func processExitCode(err error) (int, bool) {
	ee, ok := err.(*exec.ExitError)
	if !ok || ee.ProcessState == nil {
		return 0, false
	}
	ws, ok := ee.ProcessState.Sys().(syscall.WaitStatus)
	if !ok {
		return 0, false
	}
	if ws.Exited() {
		return ws.ExitStatus(), true
	}
	if ws.Signaled() {
		return 128 + int(ws.Signal()), true
	}
	return 0, false
}
