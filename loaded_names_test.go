package every

import (
	"reflect"
	"strings"
	"testing"
)

func TestLoadedNamesTestLaunchdParsesOnlyEveryLabels(t *testing.T) {
	out := "PID\tStatus\tLabel\n1234\t0\tcom.every.backup\n-\t0\tcom.every.sync-notes\n5678\t0\tcom.apple.Finder\n-\t0\tcom.every.db.dump\n"
	want := []string{"backup", "sync-notes", "db.dump"}
	if got := ParseLaunchdLabels(out); !reflect.DeepEqual(got, want) {
		t.Fatalf("got=%#v", got)
	}
}
func TestLoadedNamesTestLaunchdEmpty(t *testing.T) {
	if got := ParseLaunchdLabels(""); len(got) != 0 {
		t.Fatal(got)
	}
}
func TestLoadedNamesTestSystemdParsesOnlyEveryTimers(t *testing.T) {
	out := "every-backup.timer    loaded active waiting Timer every backup\nevery-sync-notes.timer loaded active waiting Timer every sync-notes\nother.timer           loaded active waiting Some other timer\n"
	want := []string{"backup", "sync-notes"}
	if got := ParseSystemdUnits(out); !reflect.DeepEqual(got, want) {
		t.Fatalf("got=%#v", got)
	}
}
func TestLoadedNamesTestSystemdEmpty(t *testing.T) {
	if got := ParseSystemdUnits(""); len(got) != 0 {
		t.Fatal(got)
	}
}
func TestLoadedNamesTestSystemdLeadingWhitespace(t *testing.T) {
	if got := ParseSystemdUnits("  every-backup.timer loaded active waiting d\n"); !reflect.DeepEqual(got, []string{"backup"}) {
		t.Fatal(got)
	}
}
func TestLoadedNamesTestLaunchdEnvBlockAlwaysPinsDataDir(t *testing.T) {
	block := LaunchdEnvironmentBlock()
	if !containsAll(block, "<key>EVERY_HOME</key>", DataDir()) {
		t.Fatal(block)
	}
}
func containsAll(s string, parts ...string) bool {
	for _, p := range parts {
		if !strings.Contains(s, p) {
			return false
		}
	}
	return true
}
