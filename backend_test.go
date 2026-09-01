package every

import (
	"encoding/xml"
	"strings"
	"testing"
)

func TestBackendRenderers(t *testing.T) {
	interval, _ := ParseSchedule([]string{"15m"})
	if got := SystemdTimerUnit("demo", interval); !strings.Contains(got, "OnUnitActiveSec=900") || !strings.Contains(got, "AccuracySec=1s") {
		t.Fatalf("systemd interval:\n%s", got)
	}
	calendar, _ := ParseSchedule([]string{"monday,thursday", "6pm"})
	timer := SystemdTimerUnit("demo", calendar)
	if !strings.Contains(timer, "OnCalendar=Mon *-*-* 18:00:00") || !strings.Contains(timer, "OnCalendar=Thu *-*-* 18:00:00") {
		t.Fatalf("systemd calendar:\n%s", timer)
	}
	plist := LaunchdPlist("demo", calendar)
	if !strings.Contains(plist, "StartCalendarInterval") || !strings.Contains(plist, "<integer>1</integer>") {
		t.Fatalf("launchd plist:\n%s", plist)
	}
	win := WindowsTaskXML("demo", "/usr/local/bin/every", calendar)
	if !strings.Contains(win, "<Monday/>") || !strings.Contains(win, "<Thursday/>") || !strings.Contains(win, "--home") {
		t.Fatalf("windows xml:\n%s", win)
	}
	var doc struct{ XMLName xml.Name }
	if err := xml.Unmarshal([]byte(win), &doc); err != nil {
		t.Fatalf("invalid windows xml: %v", err)
	}
}
