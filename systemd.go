package every

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

type SystemdBackend struct{}

func (SystemdBackend) dir() string { return ConfigDir() }
func (b SystemdBackend) UnitPath(name string) string {
	return filepath.Join(b.dir(), "every-"+name+".timer")
}
func (b SystemdBackend) ResourceExists(name string) bool {
	_, err := os.Stat(b.UnitPath(name))
	return err == nil
}
func (b SystemdBackend) Write(name string, s Schedule) error {
	if !Linux() {
		return unsupported("systemd")
	}
	if err := os.MkdirAll(b.dir(), 0755); err != nil {
		return err
	}
	if err := atomicWrite(filepath.Join(b.dir(), "every-"+name+".service"), SystemdServiceUnit(name)); err != nil {
		return err
	}
	return atomicWrite(b.UnitPath(name), SystemdTimerUnit(name, s))
}
func (b SystemdBackend) Enable(name string) error {
	if !Linux() {
		return unsupported("systemd")
	}
	if out, err := exec.Command("systemctl", "--user", "daemon-reload").CombinedOutput(); err != nil {
		return fmt.Errorf("systemctl daemon-reload: %s", strings.TrimSpace(string(out)))
	}
	out, err := exec.Command("systemctl", "--user", "enable", "--now", "every-"+name+".timer").CombinedOutput()
	if err != nil {
		return fmt.Errorf("systemctl enable: %s", strings.TrimSpace(string(out)))
	}
	return nil
}
func (b SystemdBackend) Disable(name string) error {
	if !Linux() {
		return unsupported("systemd")
	}
	_ = exec.Command("systemctl", "--user", "disable", "--now", "every-"+name+".timer").Run()
	return nil
}
func (b SystemdBackend) Delete(name string) error {
	_ = b.Disable(name)
	_ = os.Remove(filepath.Join(b.dir(), "every-"+name+".service"))
	return os.Remove(b.UnitPath(name))
}
func (b SystemdBackend) LoadedNames() ([]string, error) {
	if !Linux() {
		return nil, unsupported("systemd")
	}
	out, err := exec.Command("systemctl", "--user", "list-timers", "--all", "--no-legend", "--no-pager").CombinedOutput()
	if err != nil {
		return nil, err
	}
	return ParseSystemdUnits(string(out)), nil
}

func ParseSystemdUnits(out string) []string {
	var names []string
	for _, line := range strings.Split(out, "\n") {
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		unit := fields[0]
		if strings.HasPrefix(unit, "every-") && strings.HasSuffix(unit, ".timer") {
			names = append(names, strings.TrimSuffix(strings.TrimPrefix(unit, "every-"), ".timer"))
		}
	}
	return names
}
func duration(sec int64) string {
	return strconv.FormatInt(sec, 10)
}
func systemdEscape(s string) string {
	return strings.ReplaceAll(strings.ReplaceAll(s, "\\", "\\\\"), " ", "\\s")
}
func systemdQuote(s string) string {
	return `"` + strings.ReplaceAll(strings.ReplaceAll(s, `\`, `\\`), `"`, `\"`) + `"`
}
func calendarLines(s Schedule) []string {
	days := []string{"Sun", "Mon", "Tue", "Wed", "Thu", "Fri", "Sat"}
	out := []string{}
	for _, e := range s.Entries {
		prefix := "*-*-*"
		if e.Weekday != nil {
			n := *e.Weekday % 7
			if n < 0 {
				n += 7
			}
			prefix = days[n] + " *-*-*"
		}
		out = append(out, fmt.Sprintf("%s %02d:%02d:00", prefix, e.Hour, e.Minute))
	}
	return out
}

func SystemdServiceUnit(name string) string {
	exe := ExecutablePath()
	return fmt.Sprintf("[Unit]\nDescription=every task %s\n[Service]\nType=oneshot\n%s\nExecStart=%s run %s\n", name, systemdEnvironment(DataDir()), systemdExecPath(exe), systemdQuote(name))
}

func systemdExecPath(path string) string {
	if strings.ContainsAny(path, " \t\\\"") {
		return systemdQuote(path)
	}
	return path
}

func systemdEnvironment(path string) string {
	if strings.ContainsAny(path, " \t\\\"") {
		return `Environment="EVERY_HOME=` + strings.ReplaceAll(strings.ReplaceAll(path, `\`, `\\`), `"`, `\"`) + `"`
	}
	return "Environment=EVERY_HOME=" + path
}

func SystemdTimerUnit(name string, s Schedule) string {
	var timer strings.Builder
	timer.WriteString(fmt.Sprintf("[Unit]\nDescription=every timer %s\n\n[Timer]\n", name))
	if s.Kind == "interval" {
		timer.WriteString("OnUnitActiveSec=" + duration(s.Interval) + "\nOnActiveSec=" + duration(s.Interval) + "\n")
	} else {
		for _, line := range calendarLines(s) {
			timer.WriteString("OnCalendar=" + line + "\n")
		}
		timer.WriteString("Persistent=true\n")
	}
	timer.WriteString("AccuracySec=1s\nUnit=every-" + name + ".service\n\n[Install]\nWantedBy=timers.target\n")
	return timer.String()
}
