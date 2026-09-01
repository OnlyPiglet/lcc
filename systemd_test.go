package every

import (
	"strings"
	"testing"
)

func systemdTimer(t *testing.T, raw string) string {
	t.Helper()
	return SystemdTimerUnit("demo", sched(t, raw))
}
func TestSystemdTestIntervalTimer(t *testing.T) {
	s := systemdTimer(t, "15m")
	if !containsAll(s, "OnUnitActiveSec=900", "OnActiveSec=900", "WantedBy=timers.target") || strings.Contains(s, "OnCalendar") {
		t.Fatal(s)
	}
}
func TestSystemdTestDailyCalendar(t *testing.T) {
	s := systemdTimer(t, "day 9am")
	if !containsAll(s, "OnCalendar=*-*-* 09:00:00", "Persistent=true") {
		t.Fatal(s)
	}
}
func TestSystemdTestWeeklyCalendar(t *testing.T) {
	if !strings.Contains(systemdTimer(t, "monday 10:00"), "OnCalendar=Mon *-*-* 10:00:00") || !strings.Contains(systemdTimer(t, "friday 6pm"), "OnCalendar=Fri *-*-* 18:00:00") {
		t.Fatal("weekday")
	}
}
func TestSystemdTestMultiEntryCalendar(t *testing.T) {
	s := systemdTimer(t, "weekdays 9am,6pm")
	if strings.Count(s, "OnCalendar=") != 10 || !strings.Contains(s, "OnCalendar=Mon *-*-* 09:00:00") || !strings.Contains(s, "OnCalendar=Fri *-*-* 18:00:00") {
		t.Fatal(s)
	}
}
func TestSystemdTestCalendarLinesForDaySet(t *testing.T) {
	s := sched(t, "weekends 11am")
	got := calendarLines(s)
	want := []string{"Sun *-*-* 11:00:00", "Sat *-*-* 11:00:00"}
	if len(got) != 2 || got[0] != want[0] || got[1] != want[1] {
		t.Fatalf("got=%#v", got)
	}
}
func TestSystemdTestServiceUnitQuotesPaths(t *testing.T) {
	u := SystemdServiceUnit("demo")
	if !containsAll(u, "Type=oneshot", "ExecStart=", " run \"demo\"") {
		t.Fatal(u)
	}
}
func TestSystemdTestTimerSetsTightAccuracy(t *testing.T) {
	if !strings.Contains(systemdTimer(t, "15s"), "AccuracySec=1s") || !strings.Contains(systemdTimer(t, "day 9am"), "AccuracySec=1s") {
		t.Fatal("accuracy")
	}
}
func TestSystemdTestServiceAlwaysPinsDataDir(t *testing.T) {
	if !strings.Contains(SystemdServiceUnit("demo"), "Environment=EVERY_HOME="+DataDir()) {
		t.Fatal("data dir")
	}
}
func TestSystemdTestCalendarLinesWeekdaySeven(t *testing.T) {
	d := 7
	s := Schedule{Raw: "sunday 9am", Kind: "calendar", Entries: []Entry{{Weekday: &d, Hour: 9}}}
	got := calendarLines(s)
	if len(got) != 1 || got[0] != "Sun *-*-* 09:00:00" {
		t.Fatal(got)
	}
}
