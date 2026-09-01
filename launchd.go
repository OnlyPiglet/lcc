package every

import (
	"encoding/xml"
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type LaunchdBackend struct{}

func (LaunchdBackend) dir() string {
	h, _ := os.UserHomeDir()
	return filepath.Join(h, "Library", "LaunchAgents")
}
func (b LaunchdBackend) UnitPath(name string) string {
	return filepath.Join(b.dir(), "com.every."+name+".plist")
}
func (b LaunchdBackend) ResourceExists(name string) bool {
	_, err := os.Stat(b.UnitPath(name))
	return err == nil
}
func (b LaunchdBackend) Write(name string, schedule Schedule) error {
	if !Darwin() {
		return unsupported("launchd")
	}
	if err := os.MkdirAll(b.dir(), 0755); err != nil {
		return err
	}
	p := b.UnitPath(name)
	content := b.plist(name, schedule)
	if content == "" {
		return fmt.Errorf("cannot prepare launchd executable")
	}
	return atomicWrite(p, content)
}
func (b LaunchdBackend) Enable(name string) error {
	if !Darwin() {
		return unsupported("launchd")
	}
	uid := os.Getuid()
	p := b.UnitPath(name)
	out, err := exec.Command("launchctl", "bootstrap", fmt.Sprintf("gui/%d", uid), p).CombinedOutput()
	if err != nil { // reload an existing unit
		_ = exec.Command("launchctl", "bootout", fmt.Sprintf("gui/%d", uid), p).Run()
		out, err = exec.Command("launchctl", "bootstrap", fmt.Sprintf("gui/%d", uid), p).CombinedOutput()
	}
	if err != nil {
		return fmt.Errorf("launchctl bootstrap: %s", strings.TrimSpace(string(out)))
	}
	return nil
}
func (b LaunchdBackend) Disable(name string) error {
	if !Darwin() {
		return unsupported("launchd")
	}
	p := b.UnitPath(name)
	_ = exec.Command("launchctl", "bootout", fmt.Sprintf("gui/%d", os.Getuid()), p).Run()
	return nil
}
func (b LaunchdBackend) Delete(name string) error {
	_ = b.Disable(name)
	return os.Remove(b.UnitPath(name))
}
func (b LaunchdBackend) LoadedNames() ([]string, error) {
	if !Darwin() {
		return nil, unsupported("launchd")
	}
	out, err := exec.Command("launchctl", "list").CombinedOutput()
	if err != nil {
		return nil, err
	}
	return ParseLaunchdLabels(string(out)), nil
}

func ParseLaunchdLabels(out string) []string {
	var names []string
	for _, line := range strings.Split(out, "\n") {
		parts := strings.Split(line, "\t")
		label := strings.TrimSpace(parts[len(parts)-1])
		if strings.HasPrefix(label, "com.every.") {
			names = append(names, strings.TrimPrefix(label, "com.every."))
		}
	}
	return names
}

func LaunchdEnvironmentBlock() string {
	return "<key>EnvironmentVariables</key><dict><key>EVERY_HOME</key><string>" + xmlEscape(DataDir()) + "</string></dict>"
}

type plist struct {
	XMLName xml.Name  `xml:"plist"`
	Version string    `xml:"version,attr"`
	Dict    plistDict `xml:"dict"`
}
type plistDict struct {
	XMLName xml.Name `xml:"dict"`
	Items   []any    `xml:",any"`
}

func (b LaunchdBackend) plist(name string, s Schedule) string {
	exe, err := SchedulerExecutable()
	if err != nil {
		return ""
	}
	var x strings.Builder
	x.WriteString("<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n<!DOCTYPE plist PUBLIC \"-//Apple//DTD PLIST 1.0//EN\" \"http://www.apple.com/DTDs/PropertyList-1.0.dtd\">\n<plist version=\"1.0\"><dict>\n")
	x.WriteString("<key>Label</key><string>" + xmlEscape("com.every."+name) + "</string>\n")
	x.WriteString("<key>ProgramArguments</key><array><string>" + xmlEscape(exe) + "</string><string>run</string><string>" + xmlEscape(name) + "</string></array>\n")
	if s.Kind == "interval" {
		x.WriteString("<key>StartInterval</key><integer>" + strconv.FormatInt(s.Interval, 10) + "</integer>\n")
	} else {
		x.WriteString("<key>StartCalendarInterval</key><array>")
		for _, e := range s.Entries {
			x.WriteString("<dict>")
			if e.Weekday != nil {
				x.WriteString("<key>Weekday</key><integer>" + strconv.Itoa(*e.Weekday) + "</integer>")
			}
			x.WriteString("<key>Hour</key><integer>" + strconv.Itoa(e.Hour) + "</integer><key>Minute</key><integer>" + strconv.Itoa(e.Minute) + "</integer></dict>")
		}
		x.WriteString("</array>\n")
	}
	x.WriteString(LaunchdEnvironmentBlock() + "\n")
	x.WriteString("<key>RunAtLoad</key><false/><key>ProcessType</key><string>Background</string>\n</dict></plist>\n")
	return x.String()
}

// LaunchdPlist renders the platform file without registering it. It is useful
// to inspect or test a schedule on a different operating system.
func LaunchdPlist(name string, s Schedule) string { return (LaunchdBackend{}).plist(name, s) }

func xmlEscape(s string) string {
	var b strings.Builder
	_ = xml.EscapeText(&b, []byte(s))
	return b.String()
}
func currentUser() string {
	if u, err := user.Current(); err == nil {
		return u.Username
	}
	return os.Getenv("USER")
}
func atomicWrite(path, content string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(path), filepath.Base(path)+".tmp.*")
	if err != nil {
		return err
	}
	tmp := f.Name()
	defer os.Remove(tmp)
	if _, err = f.WriteString(content); err == nil {
		err = f.Sync()
	}
	if closeErr := f.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
func launchdTime(t time.Time) string { return t.Format("2006-01-02T15:04:05") }
