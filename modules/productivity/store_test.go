package productivity

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func newStore(t *testing.T) *Store {
	d := t.TempDir()
	return &Store{TasksPath: filepath.Join(d, "tasks.json"), NotesPath: filepath.Join(d, "notes.json"), PomodoroPath: filepath.Join(d, "p.json")}
}

func TestTasksPersist(t *testing.T) {
	s := newStore(t)
	a, _ := s.AddTask("write tests")
	s.AddTask("ship v1")
	if _, err := s.AddTask("   "); err == nil {
		t.Error("empty task accepted")
	}
	s.ToggleTask(a.ID)
	s2 := &Store{TasksPath: s.TasksPath, NotesPath: s.NotesPath}
	s2.Load()
	if len(s2.Tasks) != 2 || !s2.Tasks[0].Done {
		t.Fatalf("not persisted: %+v", s2.Tasks)
	}
	n, _ := s2.ClearDone()
	if n != 1 || len(s2.Tasks) != 1 || s2.Tasks[0].ID != 2 {
		t.Fatalf("clear done: %+v", s2.Tasks)
	}
}

func TestNotes(t *testing.T) {
	s := newStore(t)
	n, err := s.SaveNote(0, "", "first line\nbody")
	if err != nil || n.Title != "first line" {
		t.Fatal(n, err)
	}
	s.SaveNote(n.ID, "renamed", "b")
	if len(s.Notes) != 1 || s.Notes[0].Title != "renamed" {
		t.Fatal(s.Notes)
	}
	s.DeleteNote(n.ID)
	if len(s.Notes) != 0 {
		t.Fatal("not deleted")
	}
}

func TestCorruptFileBackedUp(t *testing.T) {
	s := newStore(t)
	os.WriteFile(s.TasksPath, []byte("{oops"), 0o644)
	if w := s.Load(); len(w) != 1 {
		t.Fatalf("warnings %v", w)
	}
	m, _ := filepath.Glob(s.TasksPath + ".invalid-*")
	if len(m) != 1 {
		t.Fatal("no backup")
	}
}

func TestPomodoroStats(t *testing.T) {
	s := newStore(t)
	d1 := time.Date(2026, 1, 1, 10, 0, 0, 0, time.UTC)
	s.RecordPomodoro(25, d1)
	s.RecordPomodoro(25, d1)
	s.RecordPomodoro(25, d1.Add(24*time.Hour))
	if s.Pomodoro.Today != 1 || s.Pomodoro.Total != 3 || s.Pomodoro.FocusMins != 75 {
		t.Fatalf("%+v", s.Pomodoro)
	}
}
