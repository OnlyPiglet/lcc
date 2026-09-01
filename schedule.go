package every

import (
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

type Schedule struct {
	Raw      string  `json:"raw"`
	Kind     string  `json:"kind"` // interval or calendar
	Interval int64   `json:"interval,omitempty"`
	Entries  []Entry `json:"entries,omitempty"`
}

type Entry struct {
	Weekday *int `json:"weekday,omitempty"` // Sunday=0, Monday=1
	Hour    int  `json:"hour"`
	Minute  int  `json:"minute"`
}

var weekdays = map[string]int{"sunday": 0, "monday": 1, "tuesday": 2, "wednesday": 3, "thursday": 4, "friday": 5, "saturday": 6}
var daySets = map[string][]int{"weekdays": {1, 2, 3, 4, 5}, "weekends": {0, 6}}
var intervalRE = regexp.MustCompile(`^(\d+)(s|m|h)$`)
var timeRE = regexp.MustCompile(`^(\d{1,2})(?::(\d{2}))?(am|pm)?$`)

func ParseSchedule(tokens []string) (Schedule, error) {
	if len(tokens) == 0 {
		return Schedule{}, fmt.Errorf("empty schedule")
	}
	raw := strings.Join(tokens, " ")
	first := strings.ToLower(tokens[0])
	if len(tokens) == 1 {
		if first == "hourly" {
			return Schedule{Raw: raw, Kind: "interval", Interval: 3600}, nil
		}
		if m := intervalRE.FindStringSubmatch(first); m != nil {
			n, parseErr := strconv.ParseInt(m[1], 10, 64)
			if parseErr != nil {
				return Schedule{}, fmt.Errorf("interval too large")
			}
			mult := int64(1)
			if m[2] == "m" {
				mult = 60
			}
			if m[2] == "h" {
				mult = 3600
			}
			if n > (1<<63-1)/mult {
				return Schedule{}, fmt.Errorf("interval too large")
			}
			if n*mult < 10 {
				return Schedule{}, fmt.Errorf("interval too small (min 10s)")
			}
			return Schedule{Raw: raw, Kind: "interval", Interval: n * mult}, nil
		}
	}
	if len(tokens) != 2 {
		return Schedule{}, fmt.Errorf("cannot parse schedule %q (examples: 15m | hourly | day 9am,6pm | weekdays 9:30 | monday,thursday 10:00)", raw)
	}
	days, err := parseDays(first)
	if err != nil {
		return Schedule{}, err
	}
	times, err := parseTimes(tokens[1])
	if err != nil {
		return Schedule{}, err
	}
	entries := make([]Entry, 0, len(days)*len(times))
	seen := map[string]bool{}
	for _, d := range days {
		for _, tm := range times {
			e := Entry{Hour: tm[0], Minute: tm[1]}
			if d >= 0 {
				x := d
				e.Weekday = &x
			}
			key := fmt.Sprintf("%v/%d/%d", d, e.Hour, e.Minute)
			if !seen[key] {
				seen[key] = true
				entries = append(entries, e)
			}
		}
	}
	return Schedule{Raw: raw, Kind: "calendar", Entries: entries}, nil
}

// UnmarshalJSON keeps task registries written by the pre-0.2 Ruby version
// readable. Those records used kind=daily/weekly plus top-level hour/minute.
func (s *Schedule) UnmarshalJSON(data []byte) error {
	var v struct {
		Raw      string  `json:"raw"`
		Kind     string  `json:"kind"`
		Interval int64   `json:"interval"`
		Entries  []Entry `json:"entries"`
		Hour     *int    `json:"hour"`
		Minute   *int    `json:"minute"`
		Weekday  *int    `json:"weekday"`
	}
	if err := json.Unmarshal(data, &v); err != nil {
		return err
	}
	s.Raw, s.Kind, s.Interval, s.Entries = v.Raw, v.Kind, v.Interval, v.Entries
	switch v.Kind {
	case "daily":
		if v.Hour == nil || v.Minute == nil {
			return fmt.Errorf("daily schedule is missing hour/minute")
		}
		s.Kind = "calendar"
		s.Entries = []Entry{{Hour: *v.Hour, Minute: *v.Minute}}
	case "weekly":
		if v.Hour == nil || v.Minute == nil || v.Weekday == nil {
			return fmt.Errorf("weekly schedule is missing weekday/hour/minute")
		}
		d := *v.Weekday % 7
		if d < 0 {
			d += 7
		}
		s.Kind = "calendar"
		s.Entries = []Entry{{Weekday: &d, Hour: *v.Hour, Minute: *v.Minute}}
	default:
		if s.Kind == "calendar" {
			*s = s.Normalize()
		}
	}
	return nil
}

func parseDays(spec string) ([]int, error) {
	if spec == "day" || spec == "daily" {
		return []int{-1}, nil
	}
	if ds, ok := daySets[spec]; ok {
		return append([]int(nil), ds...), nil
	}
	parts := strings.Split(spec, ",")
	if len(parts) == 0 {
		return nil, fmt.Errorf("cannot parse days %q", spec)
	}
	out := make([]int, 0, len(parts))
	seen := map[int]bool{}
	for _, p := range parts {
		d, ok := weekdays[p]
		if !ok {
			return nil, fmt.Errorf("cannot parse days %q (day | weekdays | weekends | monday | monday,thursday)", spec)
		}
		if !seen[d] {
			seen[d] = true
			out = append(out, d)
		}
	}
	return out, nil
}

func parseTimes(spec string) ([][2]int, error) {
	parts := strings.Split(spec, ",")
	if len(parts) == 0 {
		return nil, fmt.Errorf("cannot parse time %q", spec)
	}
	out := make([][2]int, 0, len(parts))
	seen := map[[2]int]bool{}
	for _, p := range parts {
		if p == "" {
			return nil, fmt.Errorf("cannot parse time %q (e.g. 9am or 9am,6pm)", spec)
		}
		t, err := parseTime(p)
		if err != nil {
			return nil, err
		}
		if !seen[t] {
			seen[t] = true
			out = append(out, t)
		}
	}
	return out, nil
}

func parseTime(s string) ([2]int, error) {
	m := timeRE.FindStringSubmatch(strings.ToLower(s))
	if m == nil {
		return [2]int{}, fmt.Errorf("cannot parse time %q", s)
	}
	h, _ := strconv.Atoi(m[1])
	min := 0
	if m[2] != "" {
		min, _ = strconv.Atoi(m[2])
	}
	if m[3] != "" {
		if h < 1 || h > 12 {
			return [2]int{}, fmt.Errorf("hour out of range for am/pm: %q", s)
		}
		if m[3] == "pm" && h < 12 {
			h += 12
		}
		if m[3] == "am" && h == 12 {
			h = 0
		}
	}
	if h > 23 || min > 59 {
		return [2]int{}, fmt.Errorf("time out of range: %q", s)
	}
	return [2]int{h, min}, nil
}

func (s Schedule) NextRun(from time.Time) *time.Time {
	if s.Kind == "interval" || len(s.Entries) == 0 {
		return nil
	}
	var best *time.Time
	for _, e := range s.Entries {
		t := nextEntry(e, from)
		if best == nil || t.Before(*best) {
			cp := t
			best = &cp
		}
	}
	return best
}

func nextEntry(e Entry, from time.Time) time.Time {
	t := time.Date(from.Year(), from.Month(), from.Day(), e.Hour, e.Minute, 0, 0, from.Location())
	if e.Weekday != nil {
		weekday := *e.Weekday % 7
		if weekday < 0 {
			weekday += 7
		}
		delta := (weekday - int(from.Weekday()) + 7) % 7
		t = t.AddDate(0, 0, delta)
		if !t.After(from) {
			t = t.AddDate(0, 0, 7)
		}
	} else if !t.After(from) {
		t = t.AddDate(0, 0, 1)
	}
	return t
}

func (s Schedule) HumanInterval() string {
	if s.Interval == 0 {
		return ""
	}
	if s.Interval%3600 == 0 {
		return fmt.Sprintf("%dh", s.Interval/3600)
	}
	if s.Interval%60 == 0 {
		return fmt.Sprintf("%dm", s.Interval/60)
	}
	return fmt.Sprintf("%ds", s.Interval)
}

func (s Schedule) Normalize() Schedule {
	if s.Kind != "calendar" {
		return s
	}
	out := make([]Entry, len(s.Entries))
	copy(out, s.Entries)
	for i := range out {
		if out[i].Weekday != nil {
			n := *out[i].Weekday % 7
			if n < 0 {
				n += 7
			}
			out[i].Weekday = &n
		}
	}
	s.Entries = out
	return s
}

// SortEntries gives deterministic backend output without changing semantics.
func (s Schedule) SortEntries() Schedule {
	sort.Slice(s.Entries, func(i, j int) bool {
		if s.Entries[i].Hour != s.Entries[j].Hour {
			return s.Entries[i].Hour < s.Entries[j].Hour
		}
		return s.Entries[i].Minute < s.Entries[j].Minute
	})
	return s
}
