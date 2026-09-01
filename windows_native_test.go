//go:build windows

package every

import (
	"encoding/xml"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestWindowsNativeShellDefaultsToCmd(t *testing.T) {
	shell, args := ShellCommandFor("windows", "", "echo hello")
	if !strings.EqualFold(filepath.Base(shell), "cmd.exe") {
		t.Fatalf("shell=%q", shell)
	}
	if len(args) < 3 || args[0] != "/d" || args[1] != "/s" || args[2] != "/c" {
		t.Fatalf("args=%#v", args)
	}
}

func TestWindowsNativePowerShellOverride(t *testing.T) {
	shell, args := ShellCommandFor("windows", "powershell.exe", "Write-Output hello")
	if !strings.EqualFold(filepath.Base(shell), "powershell.exe") {
		t.Fatalf("shell=%q", shell)
	}
	want := []string{"-NoLogo", "-NoProfile", "-NonInteractive", "-Command", "Write-Output hello"}
	if len(args) != len(want) {
		t.Fatalf("args=%#v", args)
	}
	for i := range want {
		if args[i] != want[i] {
			t.Fatalf("args=%#v", args)
		}
	}
}

func TestWindowsNativeCaptureEcho(t *testing.T) {
	out, code, err := Capture("echo every-windows", t.TempDir(), nil)
	if err != nil || code != 0 || !strings.Contains(strings.ToLower(string(out)), "every-windows") {
		t.Fatalf("out=%q code=%d err=%v", out, code, err)
	}
}

func TestWindowsNativeCapturePreservesQuotedArguments(t *testing.T) {
	out, code, err := Capture(`echo "quoted value"`, t.TempDir(), nil)
	if err != nil || code != 0 || !strings.Contains(string(out), `"quoted value"`) {
		t.Fatalf("out=%q code=%d err=%v", out, code, err)
	}
}

func TestWindowsNativeCaptureExitCode(t *testing.T) {
	_, code, err := Capture("exit /b 7", t.TempDir(), nil)
	if err != nil || code != 7 {
		t.Fatalf("code=%d err=%v", code, err)
	}
}

func TestWindowsNativeCaptureTimeoutKillsProcess(t *testing.T) {
	if _, err := exec.LookPath("powershell.exe"); err != nil {
		t.Skip("PowerShell is not installed")
	}
	seconds := 1.0
	started := time.Now()
	out, code, err := Capture(`powershell.exe -NoProfile -NonInteractive -Command "Start-Sleep -Seconds 30"`, t.TempDir(), &seconds)
	if err != nil || code != 124 || time.Since(started) > 8*time.Second || !strings.Contains(string(out), "timeout") {
		t.Fatalf("out=%q code=%d elapsed=%s err=%v", out, code, time.Since(started), err)
	}
}

func TestWindowsNativeTaskXMLIsWellFormed(t *testing.T) {
	s, err := ParseSchedule([]string{"monday,thursday", "6pm"})
	if err != nil {
		t.Fatal(err)
	}
	text := WindowsTaskXML("demo", `C:\Tools\every.exe`, s)
	if !strings.Contains(text, `<Monday/>`) || !strings.Contains(text, `<Thursday/>`) || !strings.Contains(text, `--home`) {
		t.Fatalf("xml=%s", text)
	}
	var document struct{ XMLName xml.Name }
	if err := xml.Unmarshal([]byte(text), &document); err != nil {
		t.Fatalf("invalid task XML: %v", err)
	}
}

func TestWindowsNativeTaskSchedulerWriteUsesEveryHome(t *testing.T) {
	d := t.TempDir()
	t.Setenv("EVERY_HOME", d)
	s, err := ParseSchedule([]string{"1m"})
	if err != nil {
		t.Fatal(err)
	}
	b := WindowsBackend{}
	if err := b.Write("native-test", s); err != nil {
		t.Fatal(err)
	}
	defer os.Remove(b.UnitPath("native-test"))
	text, err := os.ReadFile(b.UnitPath("native-test"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(text), xmlEscape(d)) || !fileExists(b.UnitPath("native-test")) {
		t.Fatalf("task XML does not pin EVERY_HOME: %s", text)
	}
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func TestWindowsNativeTaskSchedulerRejectsSubminute(t *testing.T) {
	s, err := ParseSchedule([]string{"15s"})
	if err != nil {
		t.Fatal(err)
	}
	if err := ValidateWindowsSchedule(s); err == nil || !strings.Contains(err.Error(), "from 1m") {
		t.Fatalf("err=%v", err)
	}
}

func TestWindowsNativeParseTasksHandlesBOMAndDisabled(t *testing.T) {
	out := "\ufeff\\every\\backup,08/31/2026 09:00:00,Ready\r\n\\every\\paused,N/A,Disabled\r\n\\Microsoft\\Windows\\Other,N/A,Ready\r\n"
	rows := ParseWindowsTasks(out)
	if len(rows) != 2 || rows[0].Name != "backup" || rows[0].Status != "Ready" || rows[1].Name != "paused" || rows[1].Status != "Disabled" {
		t.Fatalf("rows=%#v", rows)
	}
}

func TestWindowsNativeStoreAtomicReplacement(t *testing.T) {
	d := t.TempDir()
	t.Setenv("EVERY_HOME", d)
	s, err := LoadStore()
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 20; i++ {
		if err := s.Add("task", Task{Cmd: "echo", Schedule: Schedule{Raw: "1m", Kind: "interval", Interval: 60}}); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := os.Stat(filepath.Join(d, "tasks.json")); err != nil {
		t.Fatal(err)
	}
	if matches, _ := filepath.Glob(filepath.Join(d, "tasks.json.tmp*")); len(matches) != 0 {
		t.Fatalf("temporary files left behind: %v", matches)
	}
}

func TestWindowsNativePathAndTCCRules(t *testing.T) {
	if got := ResolveDataDir(map[string]string{"LOCALAPPDATA": `C:\Users\Alice\AppData\Local`}, "windows", `C:\Users\Alice`); got != filepath.Join(`C:\Users\Alice\AppData\Local`, "every") {
		t.Fatalf("data dir=%q", got)
	}
	if got := ResolveConfigDir(map[string]string{"APPDATA": `C:\Users\Alice\AppData\Roaming`}, "windows", `C:\Users\Alice`); got != filepath.Join(`C:\Users\Alice\AppData\Roaming`, "every") {
		t.Fatalf("config dir=%q", got)
	}
	if TCCProtected(`C:\Users\Alice\Documents\every.exe`, `C:\Users\Alice`, "windows") {
		t.Fatal("Windows path was incorrectly classified as macOS TCC-protected")
	}
}
