package every

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

const (
	Version   = "0.2.0-go"
	Homepage  = "https://github.com/Serhii-Leniv/every"
	Tagline   = "humane task scheduler for macOS (launchd), Linux (systemd), and Windows (Task Scheduler)"
	ExUsage   = 64
	ExNoInput = 66
)

func Windows() bool { return runtime.GOOS == "windows" }
func Darwin() bool  { return runtime.GOOS == "darwin" }
func Linux() bool   { return runtime.GOOS == "linux" }

// DataDir follows the same precedence as the reference implementation.
// EVERY_HOME is useful for tests and for installations with private state.
func DataDir() string {
	home, _ := os.UserHomeDir()
	return ResolveDataDir(environment(), runtime.GOOS, home)
}

func ConfigDir() string {
	home, _ := os.UserHomeDir()
	return ResolveConfigDir(environment(), runtime.GOOS, home)
}

func ResolveDataDir(env map[string]string, goos, home string) string {
	if v := env["EVERY_HOME"]; v != "" {
		return absoluteFrom(v, home)
	}
	if goos != "windows" {
		if v := env["XDG_DATA_HOME"]; filepath.IsAbs(v) {
			return filepath.Join(v, "every")
		}
		return filepath.Join(home, ".local", "share", "every")
	}
	local := env["LOCALAPPDATA"]
	if local == "" {
		local = filepath.Join(home, "AppData", "Local")
	}
	return filepath.Join(local, "every")
}

func ResolveConfigDir(env map[string]string, goos, home string) string {
	if v := env["XDG_CONFIG_HOME"]; filepath.IsAbs(v) {
		return filepath.Join(v, "systemd", "user")
	}
	if goos == "windows" {
		v := env["APPDATA"]
		if v == "" {
			v = filepath.Join(home, "AppData", "Roaming")
		}
		return filepath.Join(v, "every")
	}
	return filepath.Join(home, ".config", "systemd", "user")
}

func environment() map[string]string {
	env := make(map[string]string)
	for _, pair := range os.Environ() {
		if i := strings.IndexByte(pair, '='); i >= 0 {
			env[pair[:i]] = pair[i+1:]
		}
	}
	return env
}

func absoluteFrom(path, home string) string {
	if filepath.IsAbs(path) {
		return filepath.Clean(path)
	}
	if p, err := filepath.Abs(path); err == nil {
		return p
	}
	return filepath.Join(home, path)
}

func LogDir() string  { return filepath.Join(DataDir(), "logs") }
func RunsDir() string { return filepath.Join(DataDir(), "runs") }

// ExecutablePath prefers argv[0] so a symlink installed on PATH remains the
// stable scheduler entrypoint across upgrades. os.Executable may resolve that
// symlink on some operating systems.
func ExecutablePath() string {
	p := os.Args[0]
	if !filepath.IsAbs(p) {
		if found, err := exec.LookPath(p); err == nil {
			p = found
		} else {
			p = filepath.Join(mustWorkingDir(), p)
		}
	}
	return filepath.Clean(p)
}

func mustWorkingDir() string {
	p, err := os.Getwd()
	if err != nil {
		return "."
	}
	return p
}

func abs(path string) string {
	p, err := filepath.Abs(path)
	if err != nil {
		return filepath.Clean(path)
	}
	return p
}

type Task struct {
	Cmd       string   `json:"cmd"`
	Schedule  Schedule `json:"schedule"`
	CWD       string   `json:"cwd"`
	CreatedAt string   `json:"created_at"`
	Paused    bool     `json:"paused"`
	Quiet     bool     `json:"quiet"`
	Timeout   *float64 `json:"timeout,omitempty"`
}

type RunRecord struct {
	TS   string  `json:"ts"`
	Exit int     `json:"exit"`
	Dur  float64 `json:"dur"`
}

func encode(v any) ([]byte, error) {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(b, '\n'), nil
}

var ErrUnknownTask = errors.New("unknown task")

func taskError(name string) error { return fmt.Errorf("unknown task %q", name) }
