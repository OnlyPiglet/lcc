package every

import (
	"path/filepath"
	"testing"
)

func TestXdgTestEveryHomeOverridesEverything(t *testing.T) {
	if Windows() {
		t.Skip("Unix path semantics")
	}
	got := ResolveDataDir(map[string]string{"EVERY_HOME": "/custom/x", "XDG_DATA_HOME": "/xdg"}, "darwin", "/Users/Alice")
	if got != "/custom/x" {
		t.Fatal(got)
	}
}
func TestXdgTestXdgDataHome(t *testing.T) {
	if Windows() {
		t.Skip("Unix path semantics")
	}
	if got := ResolveDataDir(map[string]string{"XDG_DATA_HOME": "/xdg"}, "linux", "/home/a"); got != "/xdg/every" {
		t.Fatal(got)
	}
}
func TestXdgTestDefaultWhenNeitherSet(t *testing.T) {
	if Windows() {
		t.Skip("Unix path semantics")
	}
	if got := ResolveDataDir(nil, "linux", "/home/a"); got != "/home/a/.local/share/every" {
		t.Fatal(got)
	}
}
func TestXdgTestRelativeXdgDataHomeIgnored(t *testing.T) {
	if Windows() {
		t.Skip("Unix path semantics")
	}
	if got := ResolveDataDir(map[string]string{"XDG_DATA_HOME": "relative/path"}, "linux", "/home/a"); got != "/home/a/.local/share/every" {
		t.Fatal(got)
	}
}
func TestXdgTestConfigDirHonorsXdg(t *testing.T) {
	if Windows() {
		t.Skip("Unix path semantics")
	}
	if got := ResolveConfigDir(map[string]string{"XDG_CONFIG_HOME": "/cfg"}, "linux", "/home/a"); got != "/cfg/systemd/user" {
		t.Fatal(got)
	}
}
func TestXdgTestConfigDirDefault(t *testing.T) {
	if Windows() {
		t.Skip("Unix path semantics")
	}
	if got := ResolveConfigDir(nil, "linux", "/home/a"); got != "/home/a/.config/systemd/user" {
		t.Fatal(got)
	}
}
func TestXdgTestWindowsDataDirUsesLocalAppdata(t *testing.T) {
	if got := ResolveDataDir(map[string]string{"LOCALAPPDATA": "C:/Users/Alice/AppData/Local"}, "windows", "C:/Users/Alice"); got != filepath.Join("C:/Users/Alice/AppData/Local", "every") {
		t.Fatal(got)
	}
}
func TestXdgTestWindowsConfigDirUsesAppdata(t *testing.T) {
	if got := ResolveConfigDir(map[string]string{"APPDATA": "C:/Users/Alice/AppData/Roaming"}, "windows", "C:/Users/Alice"); got != filepath.Join("C:/Users/Alice/AppData/Roaming", "every") {
		t.Fatal(got)
	}
}
