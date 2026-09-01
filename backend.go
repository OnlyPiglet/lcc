package every

import (
	"fmt"
	"runtime"
)

type Backend interface {
	Write(name string, schedule Schedule) error
	Enable(name string) error
	Disable(name string) error
	Delete(name string) error
	LoadedNames() ([]string, error)
	UnitPath(name string) string
	ResourceExists(name string) bool
}

func CurrentBackend() Backend {
	return BackendFor(runtime.GOOS)
}

func BackendFor(goos string) Backend {
	switch goos {
	case "darwin":
		return LaunchdBackend{}
	case "windows":
		return WindowsBackend{}
	default:
		return SystemdBackend{}
	}
}

func unsupported(name string) error {
	return fmt.Errorf("%s is not supported on %s", name, runtime.GOOS)
}
