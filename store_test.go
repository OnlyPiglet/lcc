package every

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestStoreTestAddPersistsAndReloads(t *testing.T) {
	t.Setenv("EVERY_HOME", t.TempDir())
	s, e := LoadStore()
	if e != nil {
		t.Fatal(e)
	}
	if e = s.Add("a", Task{Cmd: "echo 1"}); e != nil {
		t.Fatal(e)
	}
	got, e := LoadStore()
	if e != nil || got.Tasks["a"].Cmd != "echo 1" {
		t.Fatalf("%#v %v", got, e)
	}
}
func TestStoreTestAtomicWriteLeavesNoTmpAndValidFile(t *testing.T) {
	d := t.TempDir()
	t.Setenv("EVERY_HOME", d)
	s, _ := LoadStore()
	for i := 0; i < 50; i++ {
		s.Tasks[fmt.Sprintf("t%d", i)] = Task{Cmd: fmt.Sprintf("echo %d", i)}
		if e := s.Save(); e != nil {
			t.Fatal(e)
		}
	}
	matches, _ := filepath.Glob(filepath.Join(d, "tasks.json.tmp*"))
	if len(matches) != 0 {
		t.Fatal(matches)
	}
	b, e := os.ReadFile(filepath.Join(d, "tasks.json"))
	if e != nil {
		t.Fatal(e)
	}
	var v Store
	if e = json.Unmarshal(b, &v); e != nil {
		t.Fatal(e)
	}
	if len(v.Tasks) != 50 {
		t.Fatal(len(v.Tasks))
	}
}
func TestStoreTestCorruptStoreFails(t *testing.T) {
	d := t.TempDir()
	t.Setenv("EVERY_HOME", d)
	_ = os.MkdirAll(d, 0755)
	_ = os.WriteFile(filepath.Join(d, "tasks.json"), []byte("{ not valid json"), 0644)
	if _, e := LoadStore(); e == nil {
		t.Fatal("corrupt store accepted")
	}
}
func TestStoreTestLastRunSkipsTornTrailingLine(t *testing.T) {
	d := t.TempDir()
	t.Setenv("EVERY_HOME", d)
	runs := filepath.Join(d, "runs")
	_ = os.MkdirAll(runs, 0755)
	p := filepath.Join(runs, "torn.jsonl")
	_ = os.WriteFile(p, []byte("{\"ts\":\"2026-07-01T09:00:00+03:00\",\"exit\":0}\n{\"ts\":\"2026-07-02T09:00:00+03:00\",\"exit\":7}\n{\"ts\":\"2026-07-03"), 0644)
	s := &Store{Tasks: map[string]Task{}}
	got, e := s.LastRun("torn")
	if e != nil || got == nil || got.Exit != 7 {
		t.Fatalf("%#v %v", got, e)
	}
}
