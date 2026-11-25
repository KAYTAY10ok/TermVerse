package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
)

func setup(t *testing.T) Paths {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(dir, "cfg"))
	t.Setenv("XDG_DATA_HOME", filepath.Join(dir, "data"))
	p, err := ResolvePaths()
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func TestResolvePathsXDG(t *testing.T) {
	p := setup(t)
	if !strings.HasSuffix(p.ConfigFile, filepath.Join("cfg", "termverse", "config.json")) {
		t.Errorf("config file %s", p.ConfigFile)
	}
	if !strings.HasSuffix(p.DataDir, filepath.Join("data", "termverse")) {
		t.Errorf("data dir %s", p.DataDir)
	}
}

func TestRelativeXDGIgnored(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "relative/path")
	t.Setenv("HOME", t.TempDir())
	p, err := ResolvePaths()
	if err != nil {
		t.Fatal(err)
	}
	if !filepath.IsAbs(p.ConfigDir) || strings.Contains(p.ConfigDir, "relative") {
		t.Errorf("relative XDG should be ignored: %s", p.ConfigDir)
	}
}

func TestLoadCreatesDefault(t *testing.T) {
	p := setup(t)
	c, w, err := Load(p, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(w) != 0 {
		t.Errorf("unexpected warnings %v", w)
	}
	if c.Theme != "cyberpunk" || !c.Appearance.Borders {
		t.Errorf("unexpected defaults %+v", c)
	}
	if _, err := os.Stat(p.ConfigFile); err != nil {
		t.Errorf("config not written: %v", err)
	}
}

func TestLoadInvalidJSONIsBackedUp(t *testing.T) {
	p := setup(t)
	os.MkdirAll(p.ConfigDir, 0o755)
	os.WriteFile(p.ConfigFile, []byte("{not json"), 0o644)
	c, w, err := Load(p, nil)
	if err != nil {
		t.Fatal(err)
	}
	if c.Theme != "cyberpunk" || len(w) == 0 {
		t.Fatalf("expected defaults and warning, got %v %v", c.Theme, w)
	}
	matches, _ := filepath.Glob(p.ConfigFile + ".invalid-*")
	if len(matches) != 1 {
		t.Fatalf("expected one backup, got %v", matches)
	}
	data, _ := os.ReadFile(matches[0])
	if string(data) != "{not json" {
		t.Error("backup content altered")
	}
}

func TestPartialConfigKeepsDefaults(t *testing.T) {
	p := setup(t)
	os.MkdirAll(p.ConfigDir, 0o755)
	os.WriteFile(p.ConfigFile, []byte(`{"theme":"matrix","refresh_seconds":999}`), 0o644)
	c, w, err := Load(p, func(n string) bool { return n == "matrix" || n == "cyberpunk" })
	if err != nil {
		t.Fatal(err)
	}
	if c.Theme != "matrix" {
		t.Errorf("theme %s", c.Theme)
	}
	if c.RefreshSeconds != 2 || len(w) != 1 {
		t.Errorf("refresh %d warnings %v", c.RefreshSeconds, w)
	}
	if !c.Widgets.Clock || c.Appearance.Selector != "▶" {
		t.Error("missing keys should keep defaults")
	}
}

func TestValidateUnknownThemeAndColors(t *testing.T) {
	c := Default()
	c.Theme = "nope"
	c.Appearance.PrimaryColor = "#abc"
	c.Appearance.AccentColor = "purple"
	c.Appearance.BorderStyle = "zigzag"
	w := c.Validate(func(n string) bool { return n == "cyberpunk" })
	if c.Theme != "cyberpunk" || c.Appearance.PrimaryColor != "#AABBCC" ||
		c.Appearance.AccentColor != "" || c.Appearance.BorderStyle != "rounded" {
		t.Errorf("unexpected %+v", c.Appearance)
	}
	if len(w) != 3 {
		t.Errorf("warnings %v", w)
	}
}

func TestSaveRoundTripAndReset(t *testing.T) {
	p := setup(t)
	c := Default()
	c.Theme = "nord"
	c.Widgets.Tips = false
	if err := Save(p, &c); err != nil {
		t.Fatal(err)
	}
	got, _, err := Load(p, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got.Theme != "nord" || got.Widgets.Tips {
		t.Errorf("roundtrip failed %+v", got)
	}
	r, err := Reset(p)
	if err != nil || r.Theme != "cyberpunk" {
		t.Fatalf("reset: %v %v", r, err)
	}
}

func TestBootstrapNeverOverwrites(t *testing.T) {
	p := setup(t)
	themes := fstest.MapFS{"matrix.json": {Data: []byte(`builtin`)}}
	ascii := fstest.MapFS{"default.txt": {Data: []byte(`ART`)}}
	os.MkdirAll(p.ThemesDir, 0o755)
	os.WriteFile(filepath.Join(p.ThemesDir, "matrix.json"), []byte(`custom`), 0o644)
	if err := Bootstrap(p, themes, ascii); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(filepath.Join(p.ThemesDir, "matrix.json"))
	if string(b) != "custom" {
		t.Error("custom theme overwritten")
	}
	b, _ = os.ReadFile(filepath.Join(p.ASCIIDir, "default.txt"))
	if string(b) != "ART" {
		t.Error("ascii not seeded")
	}
}

func TestASCIIPathNoEscape(t *testing.T) {
	p := Paths{ASCIIDir: "/cfg/ascii"}
	if got := p.ASCIIPath("../../etc/passwd"); got != "/cfg/ascii/etc/passwd" {
		t.Errorf("got %s", got)
	}
	if got := p.ASCIIPath("/abs/art.txt"); got != "/abs/art.txt" {
		t.Errorf("got %s", got)
	}
}
