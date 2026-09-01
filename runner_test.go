package every

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func testShellCommand(command string) string {
	if Windows() {
		return "powershell -NoProfile -NonInteractive -Command \"" + strings.ReplaceAll(command, `"`, `\"`) + "\""
	}
	return command
}
func TestRunnerTestRunLedgerSizeIsBounded(t *testing.T) {
	d := t.TempDir()
	t.Setenv("EVERY_HOME", d)
	p := filepath.Join(d, "runs", "loop.jsonl")
	_ = os.MkdirAll(filepath.Dir(p), 0755)
	maxSeen := int64(0)
	for i := 0; i < 8000; i++ {
		f, e := os.OpenFile(p, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
		if e != nil {
			t.Fatal(e)
		}
		_, _ = fmt.Fprintf(f, "{\"ts\":\"2026-01-01T00:00:0%d+03:00\",\"exit\":%d,\"dur\":0.1}\n", i%10, i%2)
		_ = f.Close()
		if e = trimRuns(p); e != nil {
			t.Fatal(e)
		}
		st, _ := os.Stat(p)
		if st.Size() > maxSeen {
			maxSeen = st.Size()
		}
	}
	if maxSeen > RunTrimBytes+1024 {
		t.Fatalf("ledger grew to %d", maxSeen)
	}
	b, _ := os.ReadFile(p)
	if lines := strings.Count(string(b), "\n"); lines >= 8000 {
		t.Fatal("ledger was not trimmed")
	}
	last := strings.Split(strings.TrimSpace(string(b)), "\n")
	var r RunRecord
	if e := json.Unmarshal([]byte(last[len(last)-1]), &r); e != nil || r.Exit != 1 {
		t.Fatalf("last=%#v err=%v", r, e)
	}
}
func TestRunnerTestSmallLedgerUntouched(t *testing.T) {
	d := t.TempDir()
	t.Setenv("EVERY_HOME", d)
	p := filepath.Join(d, "runs", "small.jsonl")
	_ = os.MkdirAll(filepath.Dir(p), 0755)
	for i := 0; i < 10; i++ {
		f, _ := os.OpenFile(p, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
		_, _ = fmt.Fprintf(f, "{\"ts\":\"t%d\",\"exit\":0}\n", i)
		_ = f.Close()
	}
	if e := trimRuns(p); e != nil {
		t.Fatal(e)
	}
	b, _ := os.ReadFile(p)
	if strings.Count(string(b), "\n") != 10 {
		t.Fatal("small ledger changed")
	}
}
func TestRunnerTestCaptureBoundsOutput(t *testing.T) {
	cmd := "dd if=/dev/zero bs=300000 count=1 2>/dev/null"
	if Windows() {
		t.Skip("portable output fixture is platform-specific")
	}
	out, code, e := Capture(cmd, ".", nil)
	if e != nil || code != 0 || len(out) >= 100*1024 || !strings.Contains(string(out), "truncated") {
		t.Fatalf("len=%d code=%d err=%v", len(out), code, e)
	}
}
func TestRunnerTestCaptureSmallOutputVerbatim(t *testing.T) {
	out, code, e := Capture(testShellCommand("printf 'hello there\\n'"), ".", nil)
	if e != nil || code != 0 || !strings.Contains(string(out), "hello there") || strings.Contains(string(out), "truncated") {
		t.Fatalf("%q %d %v", out, code, e)
	}
}
func TestRunnerTestCaptureTimeoutKills(t *testing.T) {
	sec := 1.0
	started := time.Now()
	out, code, e := Capture(testShellCommand("sleep 30"), ".", &sec)
	if e != nil || code != 124 || time.Since(started) > 5*time.Second || !strings.Contains(string(out), "timeout") {
		t.Fatalf("%q %d %v", out, code, e)
	}
}
func TestRunnerTestCaptureTimeoutKillsChildren(t *testing.T) {
	if Windows() {
		t.Skip("POSIX process-group fixture")
	}
	d := t.TempDir()
	marker := filepath.Join(d, "child-alive")
	sec := 1.0
	_, _, _ = Capture("(sleep 30 && touch "+marker+") & sleep 30", d, &sec)
	time.Sleep(300 * time.Millisecond)
	if _, e := os.Stat(marker); e == nil {
		t.Fatal("orphaned child survived timeout")
	}
}
func TestRunnerTestCaptureKeepsTailInMidBand(t *testing.T) {
	if Windows() {
		t.Skip("POSIX output fixture")
	}
	out, code, e := Capture("printf HEAD; dd if=/dev/zero bs=40000 count=1 2>/dev/null; printf TAILMARK", ".", nil)
	if e != nil || code != 0 || !strings.Contains(string(out), "HEAD") || !strings.Contains(string(out), "TAILMARK") || strings.Contains(string(out), "truncated") {
		t.Fatalf("%d %v %q", code, e, out)
	}
}
func TestRunnerTestCaptureMarksRealTruncation(t *testing.T) {
	if Windows() {
		t.Skip("POSIX output fixture")
	}
	out, _, _ := Capture("dd if=/dev/zero bs=200000 count=1 2>/dev/null", ".", nil)
	if !strings.Contains(string(out), "truncated") {
		t.Fatal("marker missing")
	}
}
func TestRunnerTestCaptureNonASCIIOverCapNoCrash(t *testing.T) {
	if Windows() {
		t.Skip("POSIX output fixture")
	}
	out, code, e := Capture("python3 -c \"print('é'*40000,end='')\"", ".", nil)
	if e != nil || code != 0 || len(out) == 0 {
		t.Fatalf("len=%d code=%d err=%v", len(out), code, e)
	}
}
func TestRunnerTestLoginShellFlagByShell(t *testing.T) {
	_, args := ShellCommandFor("darwin", "/bin/zsh", "echo hi")
	if args[0] != "-lc" {
		t.Fatal(args)
	}
	_, args = ShellCommandFor("linux", "/bin/sh", "echo hi")
	if args[0] != "-c" {
		t.Fatal(args)
	}
}
