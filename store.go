package every

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
)

type Store struct {
	Tasks map[string]Task `json:"tasks"`
}

func LoadStore() (*Store, error) {
	path := filepath.Join(DataDir(), "tasks.json")
	b, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return &Store{Tasks: map[string]Task{}}, nil
	}
	if err != nil {
		return nil, err
	}
	var s Store
	if err := json.Unmarshal(b, &s); err != nil {
		return nil, fmt.Errorf("%s is corrupted: %w", path, err)
	}
	if s.Tasks == nil {
		s.Tasks = map[string]Task{}
	}
	for n, t := range s.Tasks {
		t.Schedule = t.Schedule.Normalize()
		s.Tasks[n] = t
	}
	return &s, nil
}

func (s *Store) Save() error {
	if s.Tasks == nil {
		s.Tasks = map[string]Task{}
	}
	if err := os.MkdirAll(DataDir(), 0755); err != nil {
		return err
	}
	b, err := encode(s)
	if err != nil {
		return err
	}
	path := filepath.Join(DataDir(), "tasks.json")
	tmp, err := os.CreateTemp(DataDir(), "tasks.json.tmp.*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if _, err = tmp.Write(b); err == nil {
		err = tmp.Sync()
	}
	if closeErr := tmp.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}
	// os.Rename uses the platform's replace semantics. Removing the old file
	// first would create a window with no registry and could lose the old copy
	// if the replacement then fails.
	return os.Rename(tmpName, path)
}

func (s *Store) Add(name string, task Task) error {
	if s.Tasks == nil {
		s.Tasks = map[string]Task{}
	}
	s.Tasks[name] = task
	return s.Save()
}
func (s *Store) Update(name string, fn func(*Task)) error {
	t, ok := s.Tasks[name]
	if !ok {
		return taskError(name)
	}
	fn(&t)
	s.Tasks[name] = t
	return s.Save()
}
func (s *Store) Remove(name string) error { delete(s.Tasks, name); return s.Save() }

func (s *Store) LastRun(name string) (*RunRecord, error) {
	f, err := os.Open(filepath.Join(RunsDir(), name+".jsonl"))
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var records []RunRecord
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 4096), 2*1024*1024)
	for sc.Scan() {
		var r RunRecord
		if json.Unmarshal(sc.Bytes(), &r) == nil {
			records = append(records, r)
		}
	}
	if err := sc.Err(); err != nil && err != io.EOF {
		return nil, err
	}
	if len(records) == 0 {
		return nil, nil
	}
	return &records[len(records)-1], nil
}

func (s *Store) Names() []string {
	out := make([]string, 0, len(s.Tasks))
	for n := range s.Tasks {
		out = append(out, n)
	}
	sort.Strings(out)
	return out
}
