package theme

import (
	"os"
	"path/filepath"
	"testing"

	termverse "github.com/KAYTAY10ok/TermVerse"
	"io/fs"
)

func builtinFS(t *testing.T) fs.FS {
	t.Helper()
	sub, err := fs.Sub(termverse.Assets, "assets/themes")
	if err != nil {
		t.Fatal(err)
	}
	return sub
}

func TestBuiltinThemes(t *testing.T) {
	r, err := NewRegistry(builtinFS(t))
	if err != nil {
		t.Fatal(err)
	}
	if r.Len() != 5 {
		t.Fatalf("expected 5 built-in themes, got %d", r.Len())
	}
	for _, n := range BuiltinNames {
		th, ok := r.Get(n)
		if !ok {
			t.Fatalf("missing theme %s", n)
		}
		for i, f := range th.Colors.fields() {
			if !ValidHex(*f) {
				t.Errorf("%s: %s invalid %q", n, fieldNames[i], *f)
			}
		}
	}
}

func TestNormalizeHex(t *testing.T) {
	cases := map[string]string{"#abc": "#AABBCC", "ff0000": "#FF0000", " #00ff41 ": "#00FF41"}
	for in, want := range cases {
		got, ok := NormalizeHex(in)
		if !ok || got != want {
			t.Errorf("NormalizeHex(%q) = %q,%v want %q", in, got, ok, want)
		}
	}
	for _, bad := range []string{"", "#12", "#GGGGGG", "red"} {
		if _, ok := NormalizeHex(bad); ok {
			t.Errorf("NormalizeHex(%q) should fail", bad)
		}
	}
}

func TestLoadDirCustomAndInvalid(t *testing.T) {
	r, err := NewRegistry(builtinFS(t))
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "ocean.json"), []byte(`{"name":"Ocean","colors":{"primary":"#0077be"}}`), 0o644)
	os.WriteFile(filepath.Join(dir, "broken.json"), []byte(`{"name":`), 0o644)
	os.WriteFile(filepath.Join(dir, "badcolor.json"), []byte(`{"name":"bad","colors":{"primary":"blue"}}`), 0o644)
	warns := r.LoadDir(dir)
	if len(warns) != 2 {
		t.Fatalf("expected 2 warnings, got %v", warns)
	}
	th, ok := r.Get("ocean")
	if !ok {
		t.Fatal("custom theme not loaded")
	}
	if th.Colors.Primary != "#0077BE" {
		t.Errorf("primary = %s", th.Colors.Primary)
	}
	if th.Colors.Text == "" {
		t.Error("missing colors should be filled from fallback")
	}
	names := r.Names()
	if names[0] != "cyberpunk" || names[len(names)-1] != "ocean" {
		t.Errorf("unexpected order %v", names)
	}
}

func TestApplyOverrides(t *testing.T) {
	th := Theme{Colors: Colors{Primary: "#111111", Secondary: "#222222"}}
	out := th.Apply(Overrides{Primary: "#ff0000", Secondary: "nope"})
	if out.Colors.Primary != "#FF0000" || out.Colors.Secondary != "#222222" {
		t.Errorf("unexpected %+v", out.Colors)
	}
}
