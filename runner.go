package every

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"
)

const (
	MaxLogBytes   = 5 * 1024 * 1024
	MaxRunRecords = 500
	RunTrimBytes  = 256 * 1024
	HalfOutput    = 32 * 1024
)

type boundedOutput struct {
	mu         sync.Mutex
	head, tail []byte
	dropped    int64
}

func (b *boundedOutput) Write(p []byte) (int, error) {
	original := len(p)
	b.mu.Lock()
	defer b.mu.Unlock()
	if len(b.head) < HalfOutput {
		n := HalfOutput - len(b.head)
		if n > len(p) {
			n = len(p)
		}
		b.head = append(b.head, p[:n]...)
		p = p[n:]
	}
	if len(p) == 0 {
		return original, nil
	}
	b.tail = append(b.tail, p...)
	if len(b.tail) > HalfOutput {
		over := len(b.tail) - HalfOutput
		b.dropped += int64(over)
		b.tail = append([]byte(nil), b.tail[over:]...)
	}
	return original, nil
}

func (b *boundedOutput) Bytes() []byte {
	b.mu.Lock()
	defer b.mu.Unlock()
	out := append([]byte(nil), b.head...)
	if len(b.tail) > 0 {
		if b.dropped > 0 {
			out = append(out, []byte(fmt.Sprintf("\n... [%d bytes truncated] ...\n", b.dropped))...)
		}
		out = append(out, b.tail...)
	}
	return out
}

func shellCommand(command string) (string, []string) {
	if runtime.GOOS == "windows" {
		shell := os.Getenv("EVERY_SHELL")
		if shell == "" {
			shell = os.Getenv("COMSPEC")
		}
		if shell == "" {
			shell = "cmd.exe"
		}
		if isPowerShell(shell) {
			return shell, []string{"-NoLogo", "-NoProfile", "-NonInteractive", "-Command", command}
		}
		return shell, []string{"/d", "/s", "/c", command}
	}
	if runtime.GOOS == "darwin" {
		return "/bin/zsh", []string{"-lc", command}
	}
	shell := os.Getenv("SHELL")
	if shell == "" {
		shell = "/bin/bash"
	}
	base := filepath.Base(shell)
	flag := "-c"
	if base == "bash" || base == "zsh" || strings.HasSuffix(base, "bash") || strings.HasSuffix(base, "zsh") {
		flag = "-lc"
	}
	return shell, []string{flag, command}
}

func isPowerShell(shell string) bool {
	base := strings.ToLower(filepath.Base(shell))
	base = strings.TrimSuffix(base, ".exe")
	return base == "powershell" || base == "pwsh"
}

// prepareShellCommand moves Windows commands into a script file. Passing a
// command containing quotes as the final argv element of `cmd /c` causes the
// Windows command-line parser to add another escaping layer, which changes
// otherwise valid user commands. A script path is an unambiguous boundary for
// both cmd.exe and PowerShell.
func prepareShellCommand(command string) (string, []string, func(), error) {
	shell, args := shellCommand(command)
	if runtime.GOOS != "windows" {
		return shell, args, func() {}, nil
	}

	suffix := ".cmd"
	if isPowerShell(shell) {
		suffix = ".ps1"
	}
	f, err := os.CreateTemp("", "every-command-*"+suffix)
	if err != nil {
		return "", nil, nil, err
	}
	path := f.Name()
	cleanup := func() { _ = os.Remove(path) }
	contents := command + "\r\n"
	if suffix == ".ps1" {
		// Windows PowerShell 5.1 reliably detects UTF-8 scripts with a BOM.
		contents = "\ufeff" + contents
	}
	if _, err := f.WriteString(contents); err != nil {
		_ = f.Close()
		cleanup()
		return "", nil, nil, err
	}
	if err := f.Close(); err != nil {
		cleanup()
		return "", nil, nil, err
	}
	if isPowerShell(shell) {
		return shell, []string{"-NoLogo", "-NoProfile", "-NonInteractive", "-File", path}, cleanup, nil
	}
	return shell, []string{"/d", "/c", path}, cleanup, nil
}

func ShellCommandFor(goos, shell, command string) (string, []string) {
	if goos == "windows" {
		if shell == "" {
			shell = "cmd.exe"
		}
		if isPowerShell(shell) {
			return shell, []string{"-NoLogo", "-NoProfile", "-NonInteractive", "-Command", command}
		}
		return shell, []string{"/d", "/s", "/c", command}
	}
	if goos == "darwin" {
		return "/bin/zsh", []string{"-lc", command}
	}
	if shell == "" {
		shell = "/bin/bash"
	}
	base := filepath.Base(shell)
	flag := "-c"
	if base == "bash" || base == "zsh" || strings.HasSuffix(base, "bash") || strings.HasSuffix(base, "zsh") {
		flag = "-lc"
	}
	return shell, []string{flag, command}
}

func Capture(command, dir string, timeout *float64) ([]byte, int, error) {
	shell, args, cleanup, err := prepareShellCommand(command)
	if err != nil {
		return nil, 1, err
	}
	defer cleanup()
	cmd := exec.Command(shell, args...)
	cmd.Dir = dir
	configureProcess(cmd)
	collector := &boundedOutput{}
	cmd.Stdout = collector
	cmd.Stderr = collector
	if err := cmd.Start(); err != nil {
		return nil, 1, err
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	timedOut := false
	var waitErr error
	if timeout != nil {
		d := time.NewTimer(time.Duration(*timeout * float64(time.Second)))
		defer d.Stop()
		select {
		case waitErr = <-done:
			return collector.Bytes(), exitCode(waitErr, false), nil
		case <-d.C:
			timedOut = true
			terminateProcess(cmd)
			waitErr = <-done
		}
	} else {
		waitErr = <-done
	}
	out := collector.Bytes()
	if timedOut {
		out = append(out, []byte(fmt.Sprintf("\n[every: killed after %gs timeout]\n", *timeout))...)
	}
	if timedOut {
		return out, 124, nil
	}
	return out, exitCode(waitErr, false), nil
}

func exitCode(err error, timedOut bool) int {
	if timedOut {
		return 124
	}
	if err == nil {
		return 0
	}
	if code, ok := processExitCode(err); ok && code >= 0 {
		return code
	}
	return 1
}

func RunTask(name string) (int, error) { code, _, err := RunTaskOutput(name); return code, err }

func RunTaskOutput(name string) (int, []byte, error) {
	store, err := LoadStore()
	if err != nil {
		return 1, nil, err
	}
	task, ok := store.Tasks[name]
	if !ok {
		return ExNoInput, nil, taskError(name)
	}
	if err := os.MkdirAll(LogDir(), 0755); err != nil {
		return 1, nil, err
	}
	if err := os.MkdirAll(RunsDir(), 0755); err != nil {
		return 1, nil, err
	}
	started := time.Now()
	mono := time.Now()
	dir := task.CWD
	note := ""
	if dir == "" {
		dir, _ = os.UserHomeDir()
	} else if st, e := os.Stat(dir); e != nil || !st.IsDir() {
		home, _ := os.UserHomeDir()
		note = fmt.Sprintf("note: cwd %s not available under scheduler; ran from %s\n", dir, home)
		dir = home
	}
	out, code, runErr := Capture(task.Cmd, dir, task.Timeout)
	if runErr != nil {
		code = 1
		out = append(out, []byte("\n[every: "+runErr.Error()+"]\n")...)
	}
	if note != "" {
		out = append([]byte(note), out...)
	}
	dur := time.Since(mono).Seconds()
	if dur < 0 {
		dur = 0
	}
	dur = math.Round(dur*100) / 100
	if err := appendLog(name, started, code, dur, out); err != nil {
		return 1, nil, err
	}
	if err := appendRun(name, RunRecord{TS: started.Format(time.RFC3339Nano), Exit: code, Dur: dur}); err != nil {
		return 1, nil, err
	}
	if code != 0 && !task.Quiet {
		notifyFailure(name, code)
	}
	return code, out, nil
}

func appendLog(name string, started time.Time, code int, dur float64, out []byte) error {
	path := filepath.Join(LogDir(), name+".log")
	if err := rotateLog(path); err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()
	if _, err = fmt.Fprintf(f, "=== %s exit=%d dur=%.2fs ===\n", started.Format("2006-01-02 15:04:05"), code, dur); err != nil {
		return err
	}
	if _, err = f.Write(out); err != nil {
		return err
	}
	if len(out) > 0 && out[len(out)-1] != '\n' {
		_, err = f.Write([]byte{'\n'})
	}
	return err
}

func rotateLog(path string) error {
	st, err := os.Stat(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	if st.Size() < MaxLogBytes {
		return nil
	}
	old := path + ".old"
	_ = os.Remove(old)
	return os.Rename(path, old)
}

func appendRun(name string, r RunRecord) error {
	path := filepath.Join(RunsDir(), name+".jsonl")
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	b, _ := json.Marshal(r)
	_, err = f.Write(append(b, '\n'))
	_ = f.Close()
	if err != nil {
		return err
	}
	return trimRuns(path)
}

func trimRuns(path string) error {
	st, err := os.Stat(path)
	if err != nil || st.Size() <= RunTrimBytes {
		return err
	}
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 4096), 2*1024*1024)
	lines := []string{}
	for sc.Scan() {
		lines = append(lines, sc.Text())
		if len(lines) > MaxRunRecords {
			lines = lines[1:]
		}
	}
	if err := sc.Err(); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), filepath.Base(path)+".tmp.*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	for _, line := range lines {
		_, _ = io.WriteString(tmp, line+"\n")
	}
	if err = tmp.Sync(); err == nil {
		err = tmp.Close()
	} else {
		_ = tmp.Close()
	}
	if err != nil {
		return err
	}
	return os.Rename(tmpName, path)
}

func notifyFailure(name string, code int) {
	msg := fmt.Sprintf("%s failed (exit %d) - every log %s", name, code, name)
	switch runtime.GOOS {
	case "darwin":
		_ = exec.Command("osascript", "-e", fmt.Sprintf("display notification %q with title \"every\"", msg)).Run()
	case "windows":
		if u := os.Getenv("USERNAME"); u != "" {
			_ = exec.Command("msg.exe", u, "/TIME:5", msg).Run()
		}
	default:
		_ = exec.Command("notify-send", "every", msg).Run()
	}
}
