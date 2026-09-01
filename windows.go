package every

import (
	"encoding/csv"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type WindowsBackend struct{}

type WindowsTaskRow struct{ Name, Status string }

func (WindowsBackend) dir() string                   { return filepath.Join(DataDir(), "tasks") }
func (b WindowsBackend) UnitPath(name string) string { return filepath.Join(b.dir(), name+".xml") }
func (b WindowsBackend) ResourceExists(name string) bool {
	if !Windows() {
		return false
	}
	return exec.Command("schtasks.exe", "/Query", "/TN", taskPath(name), "/FO", "LIST").Run() == nil
}
func (b WindowsBackend) Write(name string, s Schedule) error {
	if !Windows() {
		return unsupported("Windows Task Scheduler")
	}
	if err := ValidateWindowsSchedule(s); err != nil {
		return err
	}
	if err := os.MkdirAll(b.dir(), 0755); err != nil {
		return err
	}
	exe := ExecutablePath()
	args := "run " + quoteArg(name) + " --home " + quoteArg(DataDir())
	xml := windowsXML(name, exe, args, s)
	return atomicWrite(b.UnitPath(name), xml)
}
func (b WindowsBackend) Enable(name string) error {
	if !Windows() {
		return unsupported("Windows Task Scheduler")
	}
	out, err := exec.Command("schtasks.exe", "/Create", "/TN", taskPath(name), "/XML", b.UnitPath(name), "/F").CombinedOutput()
	if err != nil {
		return fmt.Errorf("schtasks create: %s", strings.TrimSpace(string(out)))
	}
	return nil
}
func (b WindowsBackend) Disable(name string) error {
	if !Windows() {
		return unsupported("Windows Task Scheduler")
	}
	out, err := exec.Command("schtasks.exe", "/Change", "/TN", taskPath(name), "/DISABLE").CombinedOutput()
	if err != nil {
		return fmt.Errorf("schtasks disable: %s", strings.TrimSpace(string(out)))
	}
	return nil
}
func (b WindowsBackend) Delete(name string) error {
	if !Windows() {
		return unsupported("Windows Task Scheduler")
	}
	out, err := exec.Command("schtasks.exe", "/Delete", "/TN", taskPath(name), "/F").CombinedOutput()
	if err != nil {
		return fmt.Errorf("schtasks delete: %s", strings.TrimSpace(string(out)))
	}
	if err := os.Remove(b.UnitPath(name)); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}
func (b WindowsBackend) LoadedNames() ([]string, error) {
	if !Windows() {
		return nil, unsupported("Windows Task Scheduler")
	}
	out, err := queryWindowsTaskStates()
	if err != nil {
		return nil, err
	}
	names := []string{}
	for _, row := range ParseWindowsTaskStates(out) {
		if !strings.EqualFold(row.Status, "Disabled") {
			names = append(names, row.Name)
		}
	}
	return names, nil
}

func taskPath(name string) string { return "\\every\\" + name }

func queryWindowsTaskStates() (string, error) {
	// Task Scheduler's schtasks Status column is localized. The State property
	// exposed by Get-ScheduledTask is an enum, so its values remain stable across
	// the user's display language.
	script := "Get-ScheduledTask -TaskPath '\\every\\' | ForEach-Object { \"{0}`t{1}\" -f ($_.TaskPath + $_.TaskName), $_.State }"
	powershell := "powershell.exe"
	if path, err := exec.LookPath(powershell); err == nil {
		powershell = path
	} else if path, err := exec.LookPath("pwsh.exe"); err == nil {
		powershell = path
	}
	out, err := exec.Command(powershell, "-NoLogo", "-NoProfile", "-NonInteractive", "-Command", script).CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("query Task Scheduler: %s", strings.TrimSpace(string(out)))
	}
	return string(out), nil
}

func ParseWindowsTaskStates(out string) []WindowsTaskRow {
	rows := []WindowsTaskRow{}
	for _, line := range strings.Split(out, "\n") {
		parts := strings.SplitN(strings.TrimSpace(line), "\t", 2)
		if len(parts) != 2 {
			continue
		}
		name := strings.TrimSpace(strings.TrimPrefix(parts[0], "\ufeff"))
		if !strings.HasPrefix(strings.ToLower(name), "\\every\\") {
			continue
		}
		rows = append(rows, WindowsTaskRow{Name: name[len("\\every\\"):], Status: strings.TrimSpace(parts[1])})
	}
	return rows
}

func ParseWindowsTasks(out string) []WindowsTaskRow {
	rows, err := csv.NewReader(strings.NewReader(out)).ReadAll()
	if err != nil {
		return nil
	}
	result := []WindowsTaskRow{}
	for _, row := range rows {
		if len(row) < 3 {
			continue
		}
		name := strings.TrimSpace(strings.TrimPrefix(row[0], "\ufeff"))
		if !strings.HasPrefix(strings.ToLower(name), "\\every\\") {
			continue
		}
		result = append(result, WindowsTaskRow{Name: name[len("\\every\\"):], Status: strings.TrimSpace(row[2])})
	}
	return result
}

func ValidateWindowsSchedule(s Schedule) error {
	if s.Kind == "interval" && s.Interval < 60 {
		return fmt.Errorf("Windows Task Scheduler supports interval schedules from 1m; %s needs a resident scheduler", s.HumanInterval())
	}
	return nil
}
func windowsXML(name, exe, args string, s Schedule) string {
	var x strings.Builder
	x.WriteString("<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n")
	// Task Scheduler accepts a normal XML declaration and namespace-qualified task.
	x.WriteString("<Task version=\"1.4\" xmlns=\"http://schemas.microsoft.com/windows/2004/02/mit/task\">\n<RegistrationInfo><Author>" + xmlEscape(currentUser()) + "</Author><URI>\\every\\" + xmlEscape(name) + "</URI></RegistrationInfo><Triggers>\n")
	if s.Kind == "interval" {
		start := time.Now().Add(time.Duration(s.Interval) * time.Second)
		x.WriteString("<TimeTrigger><StartBoundary>" + start.Format("2006-01-02T15:04:05") + "</StartBoundary><Enabled>true</Enabled><Repetition><Interval>PT" + strconv.FormatInt(s.Interval, 10) + "S</Interval><StopAtDurationEnd>false</StopAtDurationEnd></Repetition></TimeTrigger>\n")
	} else {
		for _, e := range s.Entries {
			start := nextEntry(e, time.Now())
			x.WriteString("<CalendarTrigger><StartBoundary>" + start.Format("2006-01-02T15:04:05") + "</StartBoundary><Enabled>true</Enabled>")
			if e.Weekday == nil {
				x.WriteString("<ScheduleByDay><DaysInterval>1</DaysInterval></ScheduleByDay>")
			} else {
				tags := []string{"Sunday", "Monday", "Tuesday", "Wednesday", "Thursday", "Friday", "Saturday"}
				n := *e.Weekday % 7
				if n < 0 {
					n += 7
				}
				x.WriteString("<ScheduleByWeek><DaysOfWeek><" + tags[n] + "/></DaysOfWeek><WeeksInterval>1</WeeksInterval></ScheduleByWeek>")
			}
			x.WriteString("</CalendarTrigger>\n")
		}
	}
	x.WriteString("</Triggers><Principals><Principal id=\"Author\"><UserId>" + xmlEscape(currentUser()) + "</UserId><LogonType>InteractiveToken</LogonType><RunLevel>LeastPrivilege</RunLevel></Principal></Principals><Settings><MultipleInstancesPolicy>IgnoreNew</MultipleInstancesPolicy><StartWhenAvailable>true</StartWhenAvailable><AllowStartOnDemand>true</AllowStartOnDemand><Enabled>true</Enabled><ExecutionTimeLimit>PT0S</ExecutionTimeLimit></Settings><Actions Context=\"Author\"><Exec><Command>" + xmlEscape(exe) + "</Command><Arguments>" + xmlEscape(args) + "</Arguments></Exec></Actions></Task>\n")
	return x.String()
}

// WindowsTaskXML renders Task Scheduler XML without invoking schtasks.exe.
func WindowsTaskXML(name, exe string, s Schedule) string {
	return windowsXML(name, exe, "run "+quoteArg(name)+" --home "+quoteArg(DataDir()), s)
}
func quoteArg(s string) string { return `"` + strings.ReplaceAll(s, `"`, `\"`) + `"` }
