package productivity

import (
	"fmt"
	"strings"
	"time"

	"github.com/KAYTAY10ok/TermVerse/internal/fsutil"
)

// Task is a to-do item.
type Task struct {
	ID       int64     `json:"id"`
	Title    string    `json:"title"`
	Done     bool      `json:"done"`
	Priority int       `json:"priority"` // 0 normal, 1 high, 2 urgent
	Created  time.Time `json:"created"`
	DoneAt   time.Time `json:"done_at,omitempty"`
}

// Note is a quick note.
type Note struct {
	ID      int64     `json:"id"`
	Title   string    `json:"title"`
	Body    string    `json:"body"`
	Created time.Time `json:"created"`
	Updated time.Time `json:"updated"`
}

type taskFile struct {
	Tasks []Task `json:"tasks"`
}

type noteFile struct {
	Notes []Note `json:"notes"`
}

// PomodoroStats is persisted focus statistics.
type PomodoroStats struct {
	Day       string `json:"day"`
	Today     int    `json:"today"`
	Total     int    `json:"total"`
	FocusMins int    `json:"focus_minutes"`
}

// Store persists tasks and notes as JSON files.
type Store struct {
	TasksPath, NotesPath, PomodoroPath string
	Tasks                              []Task
	Notes                              []Note
	Pomodoro                           PomodoroStats
}

// Load reads both files. Invalid files are backed up, never discarded.
func (s *Store) Load() []string {
	var warns []string
	var tf taskFile
	if _, err := fsutil.ReadJSON(s.TasksPath, &tf); err != nil {
		warns = append(warns, s.backup(s.TasksPath, err))
	}
	var nf noteFile
	if _, err := fsutil.ReadJSON(s.NotesPath, &nf); err != nil {
		warns = append(warns, s.backup(s.NotesPath, err))
	}
	_, _ = fsutil.ReadJSON(s.PomodoroPath, &s.Pomodoro)
	s.Tasks, s.Notes = tf.Tasks, nf.Notes
	return warns
}

func (s *Store) backup(path string, err error) string {
	b, berr := fsutil.BackupCorrupt(path, time.Now().Format("20060102-150405"))
	if berr != nil {
		return fmt.Sprintf("%v (backup failed: %v)", err, berr)
	}
	return fmt.Sprintf("%v — file saved as %s", err, b)
}

// SaveTasks writes tasks.json atomically.
func (s *Store) SaveTasks() error { return fsutil.WriteJSONAtomic(s.TasksPath, taskFile{s.Tasks}) }

// SaveNotes writes notes.json atomically.
func (s *Store) SaveNotes() error { return fsutil.WriteJSONAtomic(s.NotesPath, noteFile{s.Notes}) }

// SavePomodoro writes pomodoro.json atomically.
func (s *Store) SavePomodoro() error { return fsutil.WriteJSONAtomic(s.PomodoroPath, s.Pomodoro) }

func nextID[T any](items []T, id func(T) int64) int64 {
	var m int64
	for _, it := range items {
		m = max(m, id(it))
	}
	return m + 1
}

// AddTask appends a task.
func (s *Store) AddTask(title string) (Task, error) {
	title = strings.TrimSpace(title)
	if title == "" {
		return Task{}, fmt.Errorf("empty task")
	}
	t := Task{ID: nextID(s.Tasks, func(t Task) int64 { return t.ID }), Title: title, Created: time.Now()}
	s.Tasks = append(s.Tasks, t)
	return t, s.SaveTasks()
}

// ToggleTask flips the done state.
func (s *Store) ToggleTask(id int64) error {
	for i := range s.Tasks {
		if s.Tasks[i].ID == id {
			s.Tasks[i].Done = !s.Tasks[i].Done
			if s.Tasks[i].Done {
				s.Tasks[i].DoneAt = time.Now()
			} else {
				s.Tasks[i].DoneAt = time.Time{}
			}
		}
	}
	return s.SaveTasks()
}

// UpdateTask applies f to the task with id.
func (s *Store) UpdateTask(id int64, f func(*Task)) error {
	for i := range s.Tasks {
		if s.Tasks[i].ID == id {
			f(&s.Tasks[i])
		}
	}
	return s.SaveTasks()
}

// DeleteTask removes a task.
func (s *Store) DeleteTask(id int64) error {
	out := s.Tasks[:0]
	for _, t := range s.Tasks {
		if t.ID != id {
			out = append(out, t)
		}
	}
	s.Tasks = out
	return s.SaveTasks()
}

// ClearDone removes completed tasks and returns how many were removed.
func (s *Store) ClearDone() (int, error) {
	out := s.Tasks[:0]
	n := 0
	for _, t := range s.Tasks {
		if t.Done {
			n++
			continue
		}
		out = append(out, t)
	}
	s.Tasks = out
	return n, s.SaveTasks()
}

// SaveNote creates (id == 0) or updates a note.
func (s *Store) SaveNote(id int64, title, body string) (Note, error) {
	title = strings.TrimSpace(title)
	if title == "" {
		title = firstLine(body)
	}
	if title == "" {
		return Note{}, fmt.Errorf("empty note")
	}
	now := time.Now()
	for i := range s.Notes {
		if s.Notes[i].ID == id {
			s.Notes[i].Title, s.Notes[i].Body, s.Notes[i].Updated = title, body, now
			return s.Notes[i], s.SaveNotes()
		}
	}
	n := Note{ID: nextID(s.Notes, func(n Note) int64 { return n.ID }), Title: title, Body: body, Created: now, Updated: now}
	s.Notes = append(s.Notes, n)
	return n, s.SaveNotes()
}

// DeleteNote removes a note.
func (s *Store) DeleteNote(id int64) error {
	out := s.Notes[:0]
	for _, n := range s.Notes {
		if n.ID != id {
			out = append(out, n)
		}
	}
	s.Notes = out
	return s.SaveNotes()
}

// RecordPomodoro counts a completed focus session.
func (s *Store) RecordPomodoro(minutes int, now time.Time) error {
	day := now.Format("2006-01-02")
	if s.Pomodoro.Day != day {
		s.Pomodoro.Day, s.Pomodoro.Today = day, 0
	}
	s.Pomodoro.Today++
	s.Pomodoro.Total++
	s.Pomodoro.FocusMins += minutes
	return s.SavePomodoro()
}

func firstLine(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		s = s[:i]
	}
	if len([]rune(s)) > 40 {
		s = string([]rune(s)[:40])
	}
	return s
}
