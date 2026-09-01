package every

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const MaxName = 100

type CLI struct{ Args []string }

func (c CLI) Run() int {
	if len(c.Args) == 0 {
		c.help()
		return 0
	}
	switch c.Args[0] {
	case "help", "-h", "--help":
		c.help()
		return 0
	case "version", "--version":
		fmt.Printf("every %s\n%s\n%s\n", Version, Tagline, Homepage)
		return 0
	case "list", "ls":
		return c.list()
	case "log":
		return c.log()
	case "run":
		if len(c.Args) < 2 {
			return c.usage("run <name>")
		}
		name, home, err := parseRunArgs(c.Args[1:])
		if err != nil {
			return c.errorf(err.Error())
		}
		oldHome, hadHome := os.LookupEnv("EVERY_HOME")
		if home != "" {
			_ = os.Setenv("EVERY_HOME", home)
			defer func() {
				if hadHome {
					_ = os.Setenv("EVERY_HOME", oldHome)
				} else {
					_ = os.Unsetenv("EVERY_HOME")
				}
			}()
		}
		code, out, err := RunTaskOutput(name)
		if len(out) > 0 {
			_, _ = os.Stdout.Write(out)
		}
		if err != nil {
			return c.fail(err, code)
		}
		return code
	case "rm", "remove":
		if len(c.Args) < 2 {
			return c.usage("rm <name>")
		}
		return c.remove(c.Args[1])
	case "pause":
		if len(c.Args) < 2 {
			return c.usage("pause <name>")
		}
		return c.pause(c.Args[1], true)
	case "resume":
		if len(c.Args) < 2 {
			return c.usage("resume <name>")
		}
		return c.resume(c.Args[1])
	case "doctor":
		return c.doctor()
	default:
		return c.add(c.Args)
	}
}

func parseRunArgs(args []string) (string, string, error) {
	if len(args) == 0 {
		return "", "", fmt.Errorf("run needs a task name")
	}
	name := args[0]
	home := ""
	for i := 1; i < len(args); i++ {
		if args[i] == "--home" {
			if i+1 >= len(args) || args[i+1] == "" {
				return "", "", fmt.Errorf("--home needs a value")
			}
			home = args[i+1]
			i++
			continue
		}
		if strings.HasPrefix(args[i], "--home=") {
			home = strings.TrimPrefix(args[i], "--home=")
			if home == "" {
				return "", "", fmt.Errorf("--home needs a value")
			}
			continue
		}
		return "", "", fmt.Errorf("unexpected argument %q after run name", args[i])
	}
	if !validTaskName(name) {
		return "", "", fmt.Errorf("invalid task name %q", name)
	}
	return name, home, nil
}

func (c CLI) add(args []string) int {
	sep := index(args, "--")
	if sep < 0 {
		return c.errorf("%q isn't a command, and there's no `--` before a task.\n  to schedule: every <when> -- <command>", args[0])
	}
	pre := append([]string(nil), args[:sep]...)
	cmdTokens := args[sep+1:]
	if len(cmdTokens) == 0 {
		return c.errorf("missing command after --")
	}
	nameFlag := hasValueFlag(pre, "--name")
	var name, timeoutRaw string
	var err error
	pre, name, err = extractFlag(pre, "--name")
	if err != nil {
		return c.errorf(err.Error())
	}
	pre, timeoutRaw, err = extractFlag(pre, "--timeout")
	if err != nil {
		return c.errorf(err.Error())
	}
	quiet := false
	kept := pre[:0]
	for _, t := range pre {
		if t == "--quiet" {
			quiet = true
		} else {
			kept = append(kept, t)
		}
	}
	pre = kept
	s, err := ParseSchedule(pre)
	if err != nil {
		return c.errorf(err.Error())
	}
	var timeout *float64
	if timeoutRaw != "" {
		seconds, e := parseDuration(timeoutRaw)
		if e != nil {
			return c.errorf(e.Error())
		}
		timeout = &seconds
	}
	cmd := strings.Join(cmdTokens, " ")
	store, err := LoadStore()
	if err != nil {
		return c.fail(err, 1)
	}
	if nameFlag {
		name = sanitize(name)
		if name == "" {
			return c.errorf("--name is empty after sanitizing (names allow a-z 0-9 . _ -)")
		}
		if len(name) > MaxName {
			return c.errorf("--name is too long (max %d chars)", MaxName)
		}
		if _, ok := store.Tasks[name]; ok {
			return c.errorf("task %q already exists", name)
		}
	} else {
		name = deriveName(cmd, store)
	}
	attrs := Task{Cmd: cmd, Schedule: s, CWD: mustGetwd(), CreatedAt: time.Now().Format(time.RFC3339Nano), Quiet: quiet, Timeout: timeout}
	if err := resetHistory(name); err != nil {
		return c.fail(err, 1)
	}
	if err := store.Add(name, attrs); err != nil {
		return c.fail(err, 1)
	}
	backend := CurrentBackend()
	if err := backend.Write(name, s); err != nil {
		_ = store.Remove(name)
		return c.fail(fmt.Errorf("could not schedule %s: %w", name, err), 1)
	}
	if err := backend.Enable(name); err != nil {
		_ = store.Remove(name)
		_ = backend.Delete(name)
		return c.fail(fmt.Errorf("could not schedule %s: %w", name, err), 1)
	}
	fmt.Printf("%s scheduled %s: %s - %s\n", green("✓"), name, s.Raw, cmd)
	if next := s.NextRun(time.Now()); next != nil {
		fmt.Printf("  next run: %s\n", next.Format("Mon 02 Jan 15:04"))
	}
	if s.Interval > 0 {
		fmt.Printf("  runs every %s while the machine is awake\n", s.HumanInterval())
	}
	fmt.Printf("  output: runs in the background -> see it with `every log %s`\n", name)
	return 0
}

func (c CLI) list() int {
	jsonOut := false
	for _, a := range c.Args[1:] {
		if a == "--json" {
			jsonOut = true
		}
	}
	store, err := LoadStore()
	if err != nil {
		return c.fail(err, 1)
	}
	if len(store.Tasks) == 0 {
		if jsonOut {
			fmt.Println("[]")
		} else {
			fmt.Println("no tasks yet - try: every day 9am -- brew update")
		}
		return 0
	}
	loaded, _ := CurrentBackend().LoadedNames()
	set := map[string]bool{}
	for _, n := range loaded {
		set[n] = true
	}
	type rec struct {
		Name      string  `json:"name"`
		Schedule  string  `json:"schedule"`
		Command   string  `json:"command"`
		Paused    bool    `json:"paused"`
		Scheduled bool    `json:"scheduled"`
		Status    string  `json:"status"`
		Last      any     `json:"last"`
		Next      *string `json:"next"`
	}
	records := []rec{}
	for _, name := range store.Names() {
		t := store.Tasks[name]
		last, _ := store.LastRun(name)
		scheduled := !t.Paused && set[name]
		var next *string
		if scheduled {
			if n := nextISO(t.Schedule, last); n != "" {
				next = &n
			}
		}
		records = append(records, rec{Name: name, Schedule: t.Schedule.Raw, Command: t.Cmd, Paused: t.Paused, Scheduled: scheduled, Status: taskStatus(t.Paused, scheduled, last), Last: runJSON(last), Next: next})
	}
	if jsonOut {
		b, _ := json.Marshal(records)
		fmt.Println(string(b))
		return 0
	}
	rows := [][]string{{"NAME", "SCHEDULE", "LAST", "STATUS", "NEXT"}}
	for _, r := range records {
		last := "-"
		if x, ok := r.Last.(map[string]any); ok {
			if ts, ok := x["at"].(string); ok {
				if tm, e := time.Parse(time.RFC3339Nano, ts); e == nil {
					last = tm.Format("02 Jan 15:04")
				}
			}
		}
		next := "-"
		if r.Next != nil {
			if tm, e := time.Parse(time.RFC3339Nano, *r.Next); e == nil {
				next = tm.Format("02 Jan 15:04")
			}
		}
		rows = append(rows, []string{r.Name, r.Schedule, last, r.Status, next})
	}
	width := make([]int, 5)
	for _, row := range rows {
		for i, v := range row {
			if len(v) > width[i] {
				width[i] = len(v)
			}
		}
	}
	for _, row := range rows {
		for i, v := range row {
			if i > 0 {
				fmt.Print("  ")
			}
			fmt.Print(v + strings.Repeat(" ", width[i]-len(v)))
		}
		fmt.Println()
	}
	return 0
}

func (c CLI) log() int {
	args := append([]string(nil), c.Args[1:]...)
	n := 40
	for i := 0; i < len(args); i++ {
		if args[i] == "-n" {
			if i+1 >= len(args) {
				return c.usage("log <name> [-n N]")
			}
			n, _ = strconv.Atoi(args[i+1])
			if n <= 0 {
				n = 40
			}
			args = append(args[:i], args[i+2:]...)
			break
		}
	}
	if len(args) == 0 {
		return c.usage("log <name> [-n N]")
	}
	if !validTaskName(args[0]) {
		return c.errorf("invalid task name %q", args[0])
	}
	b, err := os.ReadFile(filepath.Join(LogDir(), args[0]+".log"))
	if os.IsNotExist(err) {
		fmt.Fprintf(os.Stderr, "every: no logs yet for %q (has it run? check: every list)\n", args[0])
		return ExNoInput
	}
	if err != nil {
		return c.fail(err, 1)
	}
	lines := strings.SplitAfter(string(b), "\n")
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	fmt.Print(strings.Join(lines, ""))
	return 0
}

func (c CLI) remove(name string) int {
	if !validTaskName(name) {
		return c.errorf("invalid task name %q", name)
	}
	store, err := LoadStore()
	if err != nil {
		return c.fail(err, 1)
	}
	if _, ok := store.Tasks[name]; !ok {
		fmt.Fprintf(os.Stderr, "every: no task %q\n", name)
		return ExNoInput
	}
	b := CurrentBackend()
	if err := b.Disable(name); err != nil {
		return c.fail(fmt.Errorf("could not remove %s: %w", name, err), 1)
	}
	if err := b.Delete(name); err != nil {
		return c.fail(fmt.Errorf("could not remove %s: %w", name, err), 1)
	}
	if err := store.Remove(name); err != nil {
		return c.fail(err, 1)
	}
	fmt.Printf("%s removed %s (logs kept in %s)\n", green("✓"), name, LogDir())
	return 0
}
func (c CLI) pause(name string, paused bool) int {
	if !validTaskName(name) {
		return c.errorf("invalid task name %q", name)
	}
	store, err := LoadStore()
	if err != nil {
		return c.fail(err, 1)
	}
	if _, ok := store.Tasks[name]; !ok {
		fmt.Fprintf(os.Stderr, "every: no task %q\n", name)
		return ExNoInput
	}
	if err := CurrentBackend().Disable(name); err != nil {
		return c.fail(fmt.Errorf("could not pause %s: %w", name, err), 1)
	}
	if err := store.Update(name, func(t *Task) { t.Paused = paused }); err != nil {
		return c.fail(err, 1)
	}
	fmt.Printf("%s %s %s\n", green("✓"), map[bool]string{true: "paused", false: "resumed"}[paused], name)
	return 0
}
func (c CLI) resume(name string) int {
	if !validTaskName(name) {
		return c.errorf("invalid task name %q", name)
	}
	store, err := LoadStore()
	if err != nil {
		return c.fail(err, 1)
	}
	t, ok := store.Tasks[name]
	if !ok {
		fmt.Fprintf(os.Stderr, "every: no task %q\n", name)
		return ExNoInput
	}
	b := CurrentBackend()
	if err := b.Write(name, t.Schedule); err != nil {
		return c.fail(err, 1)
	}
	if err := b.Enable(name); err != nil {
		return c.fail(err, 1)
	}
	if err := store.Update(name, func(t *Task) { t.Paused = false }); err != nil {
		return c.fail(err, 1)
	}
	fmt.Printf("%s resumed %s\n", green("✓"), name)
	return 0
}

func (c CLI) doctor() int {
	store, err := LoadStore()
	if err != nil {
		return c.fail(err, 1)
	}
	d := DataDir()
	if err := os.MkdirAll(d, 0755); err != nil {
		return c.fail(err, 1)
	}
	fmt.Printf("  %s data dir writable (%s)\n", green("✓"), d)
	loaded, _ := CurrentBackend().LoadedNames()
	set := map[string]bool{}
	for _, n := range loaded {
		set[n] = true
	}
	failures := 0
	b := CurrentBackend()
	for _, n := range store.Names() {
		t := store.Tasks[n]
		fmt.Printf("\ntask: %s\n", n)
		if b.ResourceExists(n) {
			fmt.Printf("  %s scheduler resource exists (%s)\n", green("✓"), b.UnitPath(n))
		} else {
			fmt.Printf("  %s scheduler resource missing (%s)\n", red("✗"), b.UnitPath(n))
			failures++
		}
		if t.Paused {
			fmt.Println("  - paused")
		} else if set[n] {
			fmt.Printf("  %s scheduled\n", green("✓"))
		} else {
			fmt.Printf("  %s not loaded; run every resume %s\n", red("✗"), n)
			failures++
		}
		last, _ := store.LastRun(n)
		if last == nil {
			fmt.Println("  - no runs recorded yet")
		} else if last.Exit != 0 {
			fmt.Printf("  %s last run failed (exit=%d); see every log %s\n", red("✗"), last.Exit, n)
			failures++
		} else {
			fmt.Printf("  %s last run ok (%s)\n", green("✓"), last.TS)
		}
	}
	if failures > 0 {
		return 1
	}
	fmt.Printf("\n%s all good\n", green("✓"))
	return 0
}

func (c CLI) help() {
	fmt.Printf("every %s - %s\n\nadd a task:\n  every 15m -- ~/bin/sync-notes.sh\n  every day 9am,6pm -- ruby ~/bin/report.rb\n  every weekdays 9:30 -- ~/bin/standup-prep.sh\n\nflags: --name NAME, --quiet, --timeout 30m\nmanage:\n  every list [--json]       show tasks and status\n  every log <name> [-n N]   show recent output\n  every run <name>          run now\n  every pause <name>        pause scheduling\n  every resume <name>       resume scheduling\n  every rm <name>           remove task (logs kept)\n  every doctor              diagnose scheduler state\n\n", Version, Tagline)
}

func (c CLI) usage(msg string) int { fmt.Fprintf(os.Stderr, "usage: every %s\n", msg); return ExUsage }
func (c CLI) errorf(format string, args ...any) int {
	fmt.Fprintf(os.Stderr, "every: "+format+"\nsee: every help\n", args...)
	return ExUsage
}
func (c CLI) fail(err error, code int) int {
	if err != nil {
		fmt.Fprintf(os.Stderr, "every: %s\n", err)
	}
	return code
}
func extractFlag(tokens []string, flag string) ([]string, string, error) {
	for i, t := range tokens {
		if t == flag {
			if i+1 >= len(tokens) || strings.HasPrefix(tokens[i+1], "--") {
				return nil, "", fmt.Errorf("%s needs a value", flag)
			}
			out := append([]string{}, tokens[:i]...)
			out = append(out, tokens[i+2:]...)
			return out, tokens[i+1], nil
		}
		if strings.HasPrefix(t, flag+"=") {
			v := strings.TrimPrefix(t, flag+"=")
			if v == "" {
				return nil, "", fmt.Errorf("%s needs a value", flag)
			}
			out := append([]string{}, tokens[:i]...)
			out = append(out, tokens[i+1:]...)
			return out, v, nil
		}
	}
	return tokens, "", nil
}

func hasValueFlag(tokens []string, flag string) bool {
	for _, t := range tokens {
		if t == flag || strings.HasPrefix(t, flag+"=") {
			return true
		}
	}
	return false
}
func parseDuration(raw string) (float64, error) {
	m := intervalRE.FindStringSubmatch(raw)
	if m == nil {
		return 0, fmt.Errorf("bad duration %q (e.g. 90s, 30m, 2h)", raw)
	}
	n, err := strconv.ParseInt(m[1], 10, 64)
	if err != nil {
		return 0, fmt.Errorf("bad duration %q (value is too large)", raw)
	}
	mult := int64(1)
	if m[2] == "m" {
		mult = 60
	}
	if m[2] == "h" {
		mult = 3600
	}
	if n > (int64(^uint64(0)>>1))/mult {
		return 0, fmt.Errorf("bad duration %q (value is too large)", raw)
	}
	seconds := n * mult
	if seconds <= 0 {
		return 0, fmt.Errorf("--timeout must be greater than 0")
	}
	return float64(seconds), nil
}
func sanitize(s string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(s) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '.' || r == '_' || r == '-' {
			b.WriteRune(r)
		} else {
			b.WriteByte('-')
		}
	}
	v := strings.Trim(b.String(), "-")
	if v == "." || v == ".." {
		return ""
	}
	return v
}

func validTaskName(name string) bool {
	return name != "" && sanitize(name) == name && name != "." && name != ".." && !strings.ContainsAny(name, `/\\`)
}
func deriveName(cmd string, s *Store) string {
	first := strings.Fields(cmd)
	base := "task"
	if len(first) > 0 {
		base = sanitize(filepath.Base(first[0]))
		if len(base) > MaxName {
			base = base[:MaxName]
		}
		if base == "" {
			base = "task"
		}
	}
	name := base
	for i := 2; ; i++ {
		if _, ok := s.Tasks[name]; !ok {
			break
		}
		suffix := fmt.Sprintf("-%d", i)
		cut := MaxName - len(suffix)
		if cut < 1 {
			cut = 1
		}
		name = base[:min(len(base), cut)] + suffix
	}
	return name
}
func resetHistory(name string) error {
	_ = os.Remove(filepath.Join(RunsDir(), name+".jsonl"))
	matches, _ := filepath.Glob(filepath.Join(LogDir(), name+".log*"))
	for _, p := range matches {
		_ = os.Remove(p)
	}
	return nil
}
func mustGetwd() string { v, _ := os.Getwd(); return v }
func index(xs []string, v string) int {
	for i, x := range xs {
		if x == v {
			return i
		}
	}
	return -1
}
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
func taskStatus(paused, scheduled bool, last *RunRecord) string {
	if paused {
		return "paused"
	}
	if !scheduled {
		return "unscheduled"
	}
	if last == nil {
		return "·"
	}
	if last.Exit == 0 {
		return "ok"
	}
	return fmt.Sprintf("FAIL(%d)", last.Exit)
}
func runJSON(r *RunRecord) any {
	if r == nil {
		return nil
	}
	return map[string]any{"at": r.TS, "exit": r.Exit, "seconds": r.Dur}
}
func nextISO(s Schedule, last *RunRecord) string {
	if s.Kind == "interval" {
		if last == nil {
			return ""
		}
		t, e := time.Parse(time.RFC3339Nano, last.TS)
		if e != nil {
			return ""
		}
		return t.Add(time.Duration(s.Interval) * time.Second).Format(time.RFC3339Nano)
	}
	if t := s.NextRun(time.Now()); t != nil {
		return t.Format(time.RFC3339Nano)
	}
	return ""
}
func green(s string) string {
	if !colorEnabled() {
		return s
	}
	return "\033[32m" + s + "\033[0m"
}
func red(s string) string {
	if !colorEnabled() {
		return s
	}
	return "\033[31m" + s + "\033[0m"
}

func colorEnabled() bool {
	if os.Getenv("NO_COLOR") != "" || os.Getenv("TERM") == "dumb" {
		return false
	}
	st, err := os.Stdout.Stat()
	return err == nil && st.Mode()&os.ModeCharDevice != 0
}
