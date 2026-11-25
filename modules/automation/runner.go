package automation

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/KAYTAY10ok/TermVerse/internal/fsutil"
)

// Command is a saved command shortcut.
type Command struct {
	ID          int64     `json:"id"`
	Name        string    `json:"name"`
	Command     string    `json:"command"`
	Description string    `json:"description"`
	Dir         string    `json:"dir,omitempty"`
	Created     time.Time `json:"created"`
	LastRun     time.Time `json:"last_run,omitempty"`
	LastExit    int       `json:"last_exit"`
	Runs        int       `json:"runs"`
}

type file struct {
	Commands []Command `json:"commands"`
}

// Examples are seeded on first launch. They are read-only diagnostics.
var Examples = []Command{
	{Name: "Disk usage", Command: "df -h", Description: "Free space on mounted filesystems"},
	{Name: "Memory", Command: "free -h", Description: "RAM and swap usage"},
	{Name: "Git status", Command: "git status --short --branch", Description: "Status of the repository in the working directory"},
	{Name: "Top CPU processes", Command: "ps -eo pid,user,%cpu,%mem,comm --sort=-%cpu | head -15", Description: "Processes using the most CPU"},
	{Name: "Biggest files here", Command: "du -ah . 2>/dev/null | sort -rh | head -15", Description: "Largest files below the current directory"},
}

// Store persists commands.
type Store struct {
	Path     string
	Commands []Command
}

// Load reads the file, seeding examples when it does not exist.
func (s *Store) Load() error {
	var f file
	found, err := fsutil.ReadJSON(s.Path, &f)
	if err != nil {
		if b, berr := fsutil.BackupCorrupt(s.Path, time.Now().Format("20060102-150405")); berr == nil {
			err = fmt.Errorf("%v — saved as %s", err, filepath.Base(b))
		}
		s.Commands = nil
		return err
	}
	if !found {
		now := time.Now()
		for i, e := range Examples {
			e.ID, e.Created = int64(i+1), now
			f.Commands = append(f.Commands, e)
		}
		s.Commands = f.Commands
		return s.Save()
	}
	s.Commands = f.Commands
	return nil
}

// Save writes the file atomically.
func (s *Store) Save() error { return fsutil.WriteJSONAtomic(s.Path, file{s.Commands}) }

// Upsert creates (ID 0) or updates a command.
func (s *Store) Upsert(c Command) (Command, error) {
	c.Name, c.Command = strings.TrimSpace(c.Name), strings.TrimSpace(c.Command)
	if c.Name == "" || c.Command == "" {
		return c, errors.New("name and command are required")
	}
	if c.ID != 0 {
		for i := range s.Commands {
			if s.Commands[i].ID == c.ID {
				s.Commands[i] = c
				return c, s.Save()
			}
		}
	}
	var maxID int64
	for _, x := range s.Commands {
		maxID = max(maxID, x.ID)
	}
	c.ID, c.Created = maxID+1, time.Now()
	s.Commands = append(s.Commands, c)
	return c, s.Save()
}

// Delete removes a command.
func (s *Store) Delete(id int64) error {
	out := s.Commands[:0]
	for _, c := range s.Commands {
		if c.ID != id {
			out = append(out, c)
		}
	}
	s.Commands = out
	return s.Save()
}

// Result is the outcome of a run.
type Result struct {
	CommandID int64
	Name      string
	Command   string
	Stdout    string
	Stderr    string
	ExitCode  int
	Duration  time.Duration
	Err       error
	Cancelled bool
	TimedOut  bool
	Started   time.Time
}

type limitedBuffer struct {
	buf   bytes.Buffer
	limit int
	trunc bool
	mu    sync.Mutex
}

func (l *limitedBuffer) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if room := l.limit - l.buf.Len(); room > 0 {
		if len(p) > room {
			l.buf.Write(p[:room])
			l.trunc = true
		} else {
			l.buf.Write(p)
		}
	} else {
		l.trunc = true
	}
	return len(p), nil
}

func (l *limitedBuffer) String() string {
	s := l.buf.String()
	if l.trunc {
		s += "\n… output truncated (1 MiB limit)"
	}
	return s
}

// ExpandDir resolves "~" and empty directories.
func ExpandDir(dir string) string {
	dir = strings.TrimSpace(dir)
	if dir == "" {
		wd, _ := os.Getwd()
		return wd
	}
	if strings.HasPrefix(dir, "~") {
		h, _ := os.UserHomeDir()
		return filepath.Join(h, dir[1:])
	}
	return dir
}

// Run executes c with shell -c in its own process group, with a timeout.
// Stdin is empty: interactive programs are not supported.
func Run(ctx context.Context, shell string, c Command, timeout time.Duration) Result {
	r := Result{CommandID: c.ID, Name: c.Name, Command: c.Command, Started: time.Now()}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, shell, "-c", c.Command)
	cmd.Dir = ExpandDir(c.Dir)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error {
		// Kill the whole process group (pipelines, children).
		return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
	}
	cmd.Env = append(os.Environ(), "TERMVERSE=1")
	out, errb := &limitedBuffer{limit: 1 << 20}, &limitedBuffer{limit: 1 << 20}
	cmd.Stdout, cmd.Stderr = out, errb
	err := cmd.Run()
	r.Duration = time.Since(r.Started)
	r.Stdout, r.Stderr = out.String(), errb.String()
	var ee *exec.ExitError
	switch {
	case errors.Is(ctx.Err(), context.DeadlineExceeded):
		r.TimedOut, r.ExitCode = true, -1
		r.Err = fmt.Errorf("timed out after %s", timeout)
	case errors.Is(ctx.Err(), context.Canceled):
		r.Cancelled, r.ExitCode = true, -1
		r.Err = errors.New("cancelled by user")
	case errors.As(err, &ee):
		r.ExitCode = ee.ExitCode()
	case err != nil:
		r.ExitCode, r.Err = -1, err
	}
	return r
}
