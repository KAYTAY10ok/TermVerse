package cli

import (
	"bytes"
	"io/fs"
	"path/filepath"
	"strings"
	"testing"

	termverse "github.com/KAYTAY10ok/TermVerse"
	"github.com/KAYTAY10ok/TermVerse/internal/config"
	"github.com/KAYTAY10ok/TermVerse/internal/theme"
)

func env(t *testing.T, stdin string) (Env, *bytes.Buffer, *bytes.Buffer) {
	t.Helper()
	d := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(d, "c"))
	t.Setenv("XDG_DATA_HOME", filepath.Join(d, "d"))
	p, _ := config.ResolvePaths()
	sub, _ := fs.Sub(termverse.Assets, "assets/themes")
	r, err := theme.NewRegistry(sub)
	if err != nil {
		t.Fatal(err)
	}
	var o, e bytes.Buffer
	return Env{Paths: p, Themes: r, Stdout: &o, Stderr: &e, Stdin: strings.NewReader(stdin)}, &o, &e
}

func TestNoArgsIsInteractive(t *testing.T) {
	e, _, _ := env(t, "")
	if _, inter := Run(nil, e); !inter {
		t.Fatal("expected interactive")
	}
}

func TestHelpVersionUnknown(t *testing.T) {
	e, o, errb := env(t, "")
	if c, _ := Run([]string{"--help"}, e); c != 0 || !strings.Contains(o.String(), "theme set") {
		t.Error("help")
	}
	o.Reset()
	if c, _ := Run([]string{"--version"}, e); c != 0 || !strings.Contains(o.String(), "TermVerse") {
		t.Error("version")
	}
	if c, _ := Run([]string{"bogus"}, e); c != ExitUsage || errb.Len() == 0 {
		t.Error("unknown")
	}
}

func TestThemeSetAndList(t *testing.T) {
	e, o, _ := env(t, "")
	if c, _ := Run([]string{"theme", "set", "matrix"}, e); c != 0 {
		t.Fatal(o.String())
	}
	o.Reset()
	Run([]string{"theme", "list"}, e)
	if !strings.Contains(o.String(), "* matrix") {
		t.Errorf("list: %s", o.String())
	}
	if c, _ := Run([]string{"theme", "set", "nope"}, e); c != 1 {
		t.Error("unknown theme accepted")
	}
}

func TestConfigResetAsks(t *testing.T) {
	e, o, _ := env(t, "n\n")
	if c, _ := Run([]string{"config", "reset"}, e); c != 1 || !strings.Contains(o.String(), "Aborted") {
		t.Error("should abort")
	}
	e2, _, _ := env(t, "")
	if c, _ := Run([]string{"config", "reset", "--yes"}, e2); c != 0 {
		t.Error("reset --yes failed")
	}
}

func TestSystem(t *testing.T) {
	e, o, _ := env(t, "")
	if c, _ := Run([]string{"system"}, e); c != 0 || !strings.Contains(o.String(), "Hostname:") {
		t.Error(o.String())
	}
}
