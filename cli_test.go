package every

import (
	"testing"
	"time"
)

func TestCliTestStatusReflectsSchedulerReality(t *testing.T) {
	if got := taskStatus(true, false, &RunRecord{Exit: 0}); got != "paused" {
		t.Fatal(got)
	}
	if got := taskStatus(false, false, &RunRecord{Exit: 0}); got != "unscheduled" {
		t.Fatal(got)
	}
	if got := taskStatus(false, true, nil); got != "·" {
		t.Fatal(got)
	}
	if got := taskStatus(false, true, &RunRecord{Exit: 0}); got != "ok" {
		t.Fatal(got)
	}
	if got := taskStatus(false, true, &RunRecord{Exit: 7}); got != "FAIL(7)" {
		t.Fatal(got)
	}
}
func TestCliTestUnscheduledBeatsPastOK(t *testing.T) {
	if got := taskStatus(false, false, &RunRecord{Exit: 0}); got != "unscheduled" {
		t.Fatal(got)
	}
}
func TestCliTestNextISOInterval(t *testing.T) {
	s := sched(t, "15m")
	got := nextISO(s, &RunRecord{TS: "2026-07-25T10:00:00+03:00"})
	want := time.Date(2026, 7, 25, 10, 15, 0, 0, time.FixedZone("+03", 10800))
	parsed, e := time.Parse(time.RFC3339Nano, got)
	if e != nil || !parsed.Equal(want) {
		t.Fatalf("got=%v err=%v", got, e)
	}
}
func TestCliTestNextISOIntervalWithoutRun(t *testing.T) {
	if got := nextISO(sched(t, "15m"), nil); got != "" {
		t.Fatal(got)
	}
}
func TestCliTestNextISOCalendarFuture(t *testing.T) {
	got := nextISO(sched(t, "day 9am"), nil)
	parsed, e := time.Parse(time.RFC3339Nano, got)
	if e != nil || parsed.Hour() != 9 || !parsed.After(time.Now()) {
		t.Fatalf("got=%v err=%v", got, e)
	}
}
