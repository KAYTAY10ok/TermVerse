package update

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
)

func TestCompare(t *testing.T) {
	cases := []struct {
		a, b string
		want int
	}{
		{"0.1.0", "v0.1.0", 0}, {"0.1.0", "0.2.0", -1}, {"1.0.0", "0.9.9", 1},
		{"1.0.0-rc1", "1.0.0", -1}, {"1.10.0", "1.9.0", 1}, {"v2", "1.9.9", 1},
	}
	for _, c := range cases {
		if got := Compare(c.a, c.b); got != c.want {
			t.Errorf("Compare(%s,%s)=%d want %d", c.a, c.b, got, c.want)
		}
	}
}

func TestLatest(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/repos/o/ok/releases/latest":
			w.Write([]byte(`{"tag_name":"v0.2.0","name":"v0.2.0","body":"notes"}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	APIBase = srv.URL
	r, err := Latest(context.Background(), "o/ok")
	if err != nil || r.Version() != "0.2.0" {
		t.Fatalf("%v %v", r, err)
	}
	if _, err := Latest(context.Background(), "o/none"); err != ErrNoRelease {
		t.Fatalf("expected ErrNoRelease, got %v", err)
	}
}

func TestChangelogSection(t *testing.T) {
	cl := "# Changelog\n\n## [0.2.0] - x\n- new\n\n## [0.1.0] - y\n- first\n"
	if got := ChangelogSection(cl, "0.1.0"); got != "## [0.1.0] - y\n- first" {
		t.Errorf("got %q", got)
	}
}

func TestState(t *testing.T) {
	p := filepath.Join(t.TempDir(), "state.json")
	if s := LoadState(p); !s.LastCheck.IsZero() {
		t.Error("expected zero state")
	}
	SaveState(p, State{Latest: "1.0.0"})
	if s := LoadState(p); s.Latest != "1.0.0" {
		t.Error("state not saved")
	}
}
