package every

import (
	"encoding/json"
	"testing"
	"time"
)

func sched(t *testing.T, raw string) Schedule {
	t.Helper()
	s, err := ParseSchedule(splitSchedule(raw))
	if err != nil {
		t.Fatal(err)
	}
	return s
}
func splitSchedule(raw string) []string {
	if raw == "15m" || raw == "90s" || raw == "2h" || raw == "hourly" {
		return []string{raw}
	}
	parts := []string{}
	cur := ""
	for _, r := range raw {
		if r == ' ' {
			if cur != "" {
				parts = append(parts, cur)
				cur = ""
			}
		} else {
			cur += string(r)
		}
	}
	if cur != "" {
		parts = append(parts, cur)
	}
	return parts
}
func assertEntry(t *testing.T, s Schedule, i int, weekday *int, hour, minute int) {
	t.Helper()
	e := s.Entries[i]
	if (e.Weekday == nil) != (weekday == nil) || (e.Weekday != nil && *e.Weekday != *weekday) || e.Hour != hour || e.Minute != minute {
		t.Fatalf("entry[%d]=%#v", i, e)
	}
}

func TestScheduleTestMinutes(t *testing.T) {
	s := sched(t, "15m")
	if s.Kind != "interval" || s.Interval != 900 {
		t.Fatalf("%#v", s)
	}
}
func TestScheduleTestSecondsAndHours(t *testing.T) {
	if sched(t, "90s").Interval != 90 || sched(t, "2h").Interval != 7200 {
		t.Fatal("interval conversion")
	}
}
func TestScheduleTestHourlyAlias(t *testing.T) {
	if sched(t, "hourly").Interval != 3600 {
		t.Fatal("hourly")
	}
}
func TestScheduleTestIntervalMinimum(t *testing.T) {
	if _, err := ParseSchedule([]string{"5s"}); err == nil {
		t.Fatal("accepted short interval")
	}
}
func TestScheduleTestDayAm(t *testing.T) {
	s := sched(t, "day 9am")
	if s.Kind != "calendar" {
		t.Fatal(s.Kind)
	}
	assertEntry(t, s, 0, nil, 9, 0)
}
func TestScheduleTestDay24hWithMinutes(t *testing.T) {
	assertEntry(t, sched(t, "day 17:30"), 0, nil, 17, 30)
}
func TestScheduleTestDayPmWithMinutes(t *testing.T) {
	assertEntry(t, sched(t, "day 9:05pm"), 0, nil, 21, 5)
}
func TestScheduleTestMidnightAndNoon(t *testing.T) {
	assertEntry(t, sched(t, "day 12am"), 0, nil, 0, 0)
	assertEntry(t, sched(t, "day 12pm"), 0, nil, 12, 0)
}
func TestScheduleTestWeekday(t *testing.T) {
	d := 1
	s := sched(t, "monday 10:00")
	assertEntry(t, s, 0, &d, 10, 0)
}
func TestScheduleTestWeekdayPm(t *testing.T) {
	d := 5
	assertEntry(t, sched(t, "friday 6pm"), 0, &d, 18, 0)
}
func TestScheduleTestMultipleTimesPerDay(t *testing.T) {
	s := sched(t, "day 9am,6pm")
	if len(s.Entries) != 2 {
		t.Fatal(len(s.Entries))
	}
	assertEntry(t, s, 0, nil, 9, 0)
	assertEntry(t, s, 1, nil, 18, 0)
}
func TestScheduleTestWeekdaysSet(t *testing.T) {
	s := sched(t, "weekdays 9:30")
	if len(s.Entries) != 5 {
		t.Fatal(len(s.Entries))
	}
	for i, d := range []int{1, 2, 3, 4, 5} {
		assertEntry(t, s, i, &d, 9, 30)
	}
}
func TestScheduleTestWeekendsSet(t *testing.T) {
	s := sched(t, "weekends 11am")
	d0, d6 := 0, 6
	assertEntry(t, s, 0, &d0, 11, 0)
	assertEntry(t, s, 1, &d6, 11, 0)
}
func TestScheduleTestMultipleWeekdays(t *testing.T) {
	s := sched(t, "monday,thursday 10:00")
	d1, d4 := 1, 4
	assertEntry(t, s, 0, &d1, 10, 0)
	assertEntry(t, s, 1, &d4, 10, 0)
}
func TestScheduleTestDaySetTimesProduct(t *testing.T) {
	if len(sched(t, "weekdays 9am,6pm").Entries) != 10 {
		t.Fatal("product")
	}
}
func TestScheduleTestNextRunPicksEarliestEntry(t *testing.T) {
	s := sched(t, "day 9am,6pm")
	from := time.Date(2026, 7, 24, 12, 0, 0, 0, time.Local)
	got := s.NextRun(from)
	want := time.Date(2026, 7, 24, 18, 0, 0, 0, time.Local)
	if got == nil || !got.Equal(want) {
		t.Fatalf("got=%v want=%v", got, want)
	}
}
func TestScheduleTestFromHMigratesPre02Daily(t *testing.T) {
	var s Schedule
	b := []byte(`{"raw":"day 9am","kind":"daily","hour":9,"minute":0}`)
	if err := json.Unmarshal(b, &s); err != nil {
		t.Fatal(err)
	}
	assertEntry(t, s, 0, nil, 9, 0)
}
func TestScheduleTestFromHMigratesPre02Weekly(t *testing.T) {
	var s Schedule
	b := []byte(`{"raw":"monday 10:00","kind":"weekly","hour":10,"minute":0,"weekday":1}`)
	if err := json.Unmarshal(b, &s); err != nil {
		t.Fatal(err)
	}
	d := 1
	assertEntry(t, s, 0, &d, 10, 0)
}
func TestScheduleTestRejectsGarbage(t *testing.T) {
	bad := [][]string{{"borscht"}, {"day"}, {"day", "25:00"}, {"day", "9:75"}, {"day", "9am,25:00"}, {"monday,funday", "10:00"}, {}}
	for _, x := range bad {
		if _, err := ParseSchedule(x); err == nil {
			t.Fatalf("accepted %#v", x)
		}
	}
}
func TestScheduleTestRejectsEmptyTimeList(t *testing.T) {
	for _, x := range []string{"day ,", "day 9am,", "day 9am,,6pm"} {
		if _, err := ParseSchedule(splitSchedule(x)); err == nil {
			t.Fatal(x)
		}
	}
}
func TestScheduleTestRejectsBadAmpmHours(t *testing.T) {
	for _, x := range []string{"day 13pm", "day 0am"} {
		if _, err := ParseSchedule(splitSchedule(x)); err == nil {
			t.Fatal(x)
		}
	}
}
func TestScheduleTestDedupesRepeatedTimesAndDays(t *testing.T) {
	if len(sched(t, "day 9am,9am").Entries) != 1 || len(sched(t, "monday,monday 10:00").Entries) != 1 {
		t.Fatal("not deduped")
	}
}
func TestScheduleTestFromHClampsLegacyWeekdaySeven(t *testing.T) {
	var s Schedule
	if err := json.Unmarshal([]byte(`{"raw":"sunday 9am","kind":"weekly","hour":9,"minute":0,"weekday":7}`), &s); err != nil {
		t.Fatal(err)
	}
	if s.Entries[0].Weekday == nil || *s.Entries[0].Weekday != 0 {
		t.Fatalf("%#v", s.Entries)
	}
}
func TestScheduleTestDailyNextRunTodayIfFuture(t *testing.T) {
	from := time.Date(2026, 7, 24, 8, 0, 0, 0, time.Local)
	got := sched(t, "day 9am").NextRun(from)
	if got == nil || got.Hour() != 9 || got.Day() != 24 {
		t.Fatal(got)
	}
}
func TestScheduleTestDailyNextRunTomorrowIfPast(t *testing.T) {
	from := time.Date(2026, 7, 24, 10, 0, 0, 0, time.Local)
	got := sched(t, "day 9am").NextRun(from)
	if got == nil || got.Hour() != 9 || got.Day() != 25 {
		t.Fatal(got)
	}
}
func TestScheduleTestWeeklyNextRunWithinWeek(t *testing.T) {
	from := time.Date(2026, 7, 24, 12, 0, 0, 0, time.Local)
	got := sched(t, "monday 10:00").NextRun(from)
	if got == nil || got.Weekday() != time.Monday || got.Hour() != 10 || !got.After(from) || got.Sub(from) > 7*24*time.Hour {
		t.Fatal(got)
	}
}
func TestScheduleTestWeeklySameDayFutureTime(t *testing.T) {
	from := time.Date(2026, 7, 24, 8, 0, 0, 0, time.Local)
	got := sched(t, "friday 6pm").NextRun(from)
	if got == nil || got.Day() != 24 || got.Hour() != 18 {
		t.Fatal(got)
	}
}
func TestScheduleTestIntervalHasNoCalendarNextRun(t *testing.T) {
	if sched(t, "15m").NextRun(time.Now()) != nil {
		t.Fatal("interval has calendar run")
	}
}
func TestScheduleTestToHFromHRoundTrip(t *testing.T) {
	for _, raw := range []string{"15m", "hourly", "day 9am", "monday 10:00"} {
		s := sched(t, raw)
		b, _ := json.Marshal(s)
		var got Schedule
		if err := json.Unmarshal(b, &got); err != nil {
			t.Fatal(err)
		}
		if got.Raw != s.Raw || got.Kind != s.Kind || got.Interval != s.Interval || len(got.Entries) != len(s.Entries) {
			t.Fatalf("%s => %#v", raw, got)
		}
	}
}
