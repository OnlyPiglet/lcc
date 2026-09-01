//go:build windows

package every

import (
	"os/exec"
)

func configureProcess(cmd *exec.Cmd) {}

func terminateProcess(cmd *exec.Cmd) {
	if cmd.Process == nil {
		return
	}
	// taskkill reaches descendants created by cmd.exe or PowerShell.
	_ = exec.Command("taskkill.exe", "/PID", fmtInt(cmd.Process.Pid), "/T", "/F").Run()
}

func fmtInt(n int) string {
	if n == 0 {
		return "0"
	}
	buf := [24]byte{}
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
}

func processExitCode(err error) (int, bool) {
	ee, ok := err.(*exec.ExitError)
	if !ok || ee.ProcessState == nil {
		return 0, false
	}
	return ee.ProcessState.ExitCode(), true
}
