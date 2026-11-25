package registry

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSections(t *testing.T) {
	if len(Sections) != 11 {
		t.Fatalf("expected 11 sections, got %d", len(Sections))
	}
	for i, s := range Sections {
		if s.Number != i {
			t.Errorf("section %s has number %d, want %d", s.ID, s.Number, i)
		}
	}
}

func TestDiscover(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "good"), 0o755)
	os.WriteFile(filepath.Join(dir, "good", "plugin.json"), []byte(`{"name":"Good","version":"1.0.0"}`), 0o644)
	os.MkdirAll(filepath.Join(dir, "bad"), 0o755)
	os.WriteFile(filepath.Join(dir, "bad", "plugin.json"), []byte(`{`), 0o644)
	os.MkdirAll(filepath.Join(dir, "empty"), 0o755)
	ms, err := Discover(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(ms) != 3 {
		t.Fatalf("got %d", len(ms))
	}
	errs := 0
	for _, m := range ms {
		if m.Error != "" {
			errs++
		}
	}
	if errs != 2 {
		t.Errorf("expected 2 errors, got %d: %+v", errs, ms)
	}
	none, err := Discover(filepath.Join(dir, "missing"))
	if err != nil || none != nil {
		t.Error("missing dir should be empty, no error")
	}
}
