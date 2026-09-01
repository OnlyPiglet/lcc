package every

import (
	"encoding/xml"
	"reflect"
	"strings"
	"testing"
)

func TestWindowsTaskSchedulerTestIntervalXML(t *testing.T) {
	xmlText := WindowsTaskXML("demo", "every.exe", sched(t, "15m"))
	if !containsAll(xmlText, "<TimeTrigger>", "<Interval>PT900S</Interval>", "<MultipleInstancesPolicy>IgnoreNew</MultipleInstancesPolicy>", "<StartWhenAvailable>true</StartWhenAvailable>", `\every\demo`) {
		t.Fatal(xmlText)
	}
}
func TestWindowsTaskSchedulerTestCalendarXML(t *testing.T) {
	xmlText := WindowsTaskXML("demo", "every.exe", sched(t, "day 9am"))
	if !containsAll(xmlText, "<CalendarTrigger>", "<ScheduleByDay><DaysInterval>1</DaysInterval></ScheduleByDay>", "<Command>") {
		t.Fatal(xmlText)
	}
	var v struct{ XMLName xml.Name }
	if err := xml.Unmarshal([]byte(xmlText), &v); err != nil {
		t.Fatal(err)
	}
}
func TestWindowsTaskSchedulerTestWeeklyXML(t *testing.T) {
	xmlText := WindowsTaskXML("demo", "every.exe", sched(t, "monday,thursday 6pm"))
	if !containsAll(xmlText, "<Monday/>", "<Thursday/>") || strings.Count(xmlText, "<CalendarTrigger>") != 2 {
		t.Fatal(xmlText)
	}
}
func TestWindowsTaskSchedulerTestWrapperPinsDataDir(t *testing.T) {
	xmlText := WindowsTaskXML("demo", "every.exe", sched(t, "15m"))
	if !containsAll(xmlText, "--home", DataDir()) {
		t.Fatal(xmlText)
	}
}
func TestWindowsTaskSchedulerTestParseTasksFiltersNonEvery(t *testing.T) {
	out := "\\every\\backup,08/31/2026 09:00:00,Ready\n\\every\\paused,N/A,Disabled\n\\Microsoft\\Windows\\Other,N/A,Ready\n"
	got := ParseWindowsTasks(out)
	if len(got) != 2 || got[0].Name != "backup" || got[0].Status != "Ready" || got[1].Name != "paused" || got[1].Status != "Disabled" {
		t.Fatalf("got=%#v", got)
	}
}
func TestWindowsTaskSchedulerTestParseTaskStatesFiltersNonEvery(t *testing.T) {
	out := "\\every\\backup\tReady\r\n\\every\\paused\tDisabled\r\n\\Microsoft\\Windows\\Other\tReady\r\n"
	got := ParseWindowsTaskStates(out)
	if len(got) != 2 || got[0].Name != "backup" || got[0].Status != "Ready" || got[1].Name != "paused" || got[1].Status != "Disabled" {
		t.Fatalf("got=%#v", got)
	}
}
func TestWindowsTaskSchedulerTestSubminuteIntervalsRejected(t *testing.T) {
	err := ValidateWindowsSchedule(sched(t, "15s"))
	if err == nil || !strings.Contains(err.Error(), "from 1m") {
		t.Fatal(err)
	}
}
func TestWindowsTaskSchedulerTestWindowsShellDefaultsToCmd(t *testing.T) {
	shell, args := ShellCommandFor("windows", "", "echo hi")
	if !strings.HasSuffix(strings.ToLower(shell), "cmd.exe") || !reflect.DeepEqual(args[:3], []string{"/d", "/s", "/c"}) {
		t.Fatalf("%s %#v", shell, args)
	}
}
func TestWindowsTaskSchedulerTestBackendDispatchesWindows(t *testing.T) {
	if _, ok := BackendFor("windows").(WindowsBackend); !ok {
		t.Fatalf("%T", BackendFor("windows"))
	}
}
