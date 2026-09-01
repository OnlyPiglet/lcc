package every

import "testing"

func TestRuntimeTestTCCClassification(t *testing.T) {
	tcc := []string{"/Users/me/Documents/every", "/Users/me/Desktop/tools/every", "/Users/me/Downloads/every"}
	safe := []string{"/opt/homebrew/Cellar/every/0.1.0/libexec", "/usr/local/Cellar/every/0.1.0/libexec", "/Users/me/code/every", "/Users/me/.local/share/every/runtime"}
	for _, p := range tcc {
		if !TCCProtected(p, "/Users/me", "darwin") {
			t.Fatalf("%s should be TCC-protected", p)
		}
	}
	for _, p := range safe {
		if TCCProtected(p, "/Users/me", "darwin") {
			t.Fatalf("%s should not be TCC-protected", p)
		}
	}
	for _, p := range append(tcc, safe...) {
		if TCCProtected(p, "/Users/me", "linux") {
			t.Fatalf("%s must not be TCC on Linux", p)
		}
	}
}
