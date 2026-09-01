package every

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// SchedulerExecutable returns the binary path that should be embedded in a
// native scheduler definition. launchd can be blocked by macOS TCC when the
// executable lives in Documents, Desktop, or Downloads, so keep an atomic copy
// in the application's data directory in that case.
func SchedulerExecutable() (string, error) {
	exe := ExecutablePath()
	if !Darwin() || !needsTCCCopy(exe) {
		return exe, nil
	}
	target := filepath.Join(DataDir(), "runtime", "every")
	if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
		return "", err
	}
	tmp, err := os.CreateTemp(filepath.Dir(target), "every.tmp.*")
	if err != nil {
		return "", err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	src, err := os.Open(exe)
	if err != nil {
		_ = tmp.Close()
		return "", err
	}
	_, copyErr := io.Copy(tmp, src)
	_ = src.Close()
	if copyErr == nil {
		copyErr = tmp.Chmod(0755)
	}
	if copyErr == nil {
		copyErr = tmp.Sync()
	}
	closeErr := tmp.Close()
	if copyErr == nil {
		copyErr = closeErr
	}
	if copyErr != nil {
		return "", copyErr
	}
	if err := os.Rename(tmpName, target); err != nil {
		return "", fmt.Errorf("install runtime copy: %w", err)
	}
	return target, nil
}

func needsTCCCopy(path string) bool {
	home, err := os.UserHomeDir()
	if err != nil {
		return false
	}
	rel, err := filepath.Rel(home, path)
	if err != nil || rel == "." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return false
	}
	first := strings.Split(rel, string(filepath.Separator))[0]
	return first == "Documents" || first == "Desktop" || first == "Downloads"
}

func TCCProtected(path, home, goos string) bool {
	if goos != "darwin" {
		return false
	}
	rel, err := filepath.Rel(home, path)
	if err != nil || rel == "." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return false
	}
	first := strings.Split(rel, string(filepath.Separator))[0]
	return first == "Documents" || first == "Desktop" || first == "Downloads"
}
