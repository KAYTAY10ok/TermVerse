package automation

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestStoreSeedsAndPersists(t *testing.T) {
	p := filepath.Join(t.TempDir(), "automation.json")
	s := &Store{Path: p}
	if err := s.Load(); err != nil {
		t.Fatal(err)
	}
	if len(s.Commands) != len(Examples) {
		t.Fatalf("examples not seeded: %d", len(s.Commands))
	}
	c, err := s.Upsert(Command{Name: "hello", Command: "echo hi"})
	if err != nil || c.ID == 0 {
		t.Fatal(c, err)
	}
	if _, err := s.Upsert(Command{Name: "", Command: "x"}); err == nil {
		t.Error("empty name accepted")
	}
	s2 := &Store{Path: p}
	s2.Load()
	if len(s2.Commands) != len(Examples)+1 {
		t.Fatal("not persisted")
	}
	s2.Delete(c.ID)
	if len(s2.Commands) != len(Examples) {
		t.Fatal("not deleted")
	}
}

func TestRun(t *testing.T) {
	r := Run(context.Background(), "/bin/sh", Command{Command: "echo out; echo err >&2; exit 3"}, 5*time.Second)
	if strings.TrimSpace(r.Stdout) != "out" || strings.TrimSpace(r.Stderr) != "err" || r.ExitCode != 3 || r.Err != nil {
		t.Fatalf("%+v", r)
	}
}

func TestRunTimeout(t *testing.T) {
	r := Run(context.Background(), "/bin/sh", Command{Command: "sleep 5 | cat"}, 200*time.Millisecond)
	if !r.TimedOut || r.Duration > 3*time.Second {
		t.Fatalf("%+v", r)
	}
}

func TestRunCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	go func() { time.Sleep(100 * time.Millisecond); cancel() }()
	r := Run(ctx, "/bin/sh", Command{Command: "sleep 5"}, 10*time.Second)
	if !r.Cancelled {
		t.Fatalf("%+v", r)
	}
}
