package app_test

import (
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"

	"github.com/KAYTAY10ok/TermVerse/internal/app"
	"github.com/KAYTAY10ok/TermVerse/internal/registry"
	"github.com/KAYTAY10ok/TermVerse/internal/system"
	"github.com/KAYTAY10ok/TermVerse/internal/ui"
	"github.com/KAYTAY10ok/TermVerse/modules/arcade"
	"github.com/KAYTAY10ok/TermVerse/modules/automation"
	"github.com/KAYTAY10ok/TermVerse/modules/customization"
	"github.com/KAYTAY10ok/TermVerse/modules/devtools"
	"github.com/KAYTAY10ok/TermVerse/modules/filemanager"
	"github.com/KAYTAY10ok/TermVerse/modules/network"
	"github.com/KAYTAY10ok/TermVerse/modules/plugins"
	"github.com/KAYTAY10ok/TermVerse/modules/productivity"
	"github.com/KAYTAY10ok/TermVerse/modules/settings"
	"github.com/KAYTAY10ok/TermVerse/modules/systemcenter"
	"github.com/KAYTAY10ok/TermVerse/modules/updates"
)

func factories() map[string]app.Factory {
	return map[string]app.Factory{
		"updates": updates.New, "devtools": devtools.New, "arcade": arcade.New, "system": systemcenter.New,
		"network": network.New, "files": filemanager.New, "productivity": productivity.New,
		"automation": automation.New, "customize": customization.New, "plugins": plugins.New, "settings": settings.New,
	}
}

func newModel(t *testing.T) (*app.Model, *ui.Context) {
	t.Helper()
	d := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(d, "c"))
	t.Setenv("XDG_DATA_HOME", filepath.Join(d, "d"))
	paths, reg, warns, err := app.Setup()
	if err != nil {
		t.Fatal(err)
	}
	ctx, err := app.Context(paths, reg, warns)
	if err != nil {
		t.Fatal(err)
	}
	ctx.Sys, ctx.SysOK = ctx.Collector.Collect(true), true
	return app.New(ctx, factories()), ctx
}

// drive runs a message through the model, executing returned commands that
// complete quickly (synchronous ones) up to a small depth.
func drive(m *app.Model, msg tea.Msg) {
	queue := []tea.Msg{msg}
	for i := 0; i < 30 && len(queue) > 0; i++ {
		cur := queue[0]
		queue = queue[1:]
		_, cmd := m.Update(cur)
		queue = append(queue, run(cmd)...)
	}
}

// run executes commands but skips timers and blocking I/O-heavy ones.
func run(cmd tea.Cmd) []tea.Msg {
	if cmd == nil {
		return nil
	}
	done := make(chan tea.Msg, 1)
	go func() { done <- cmd() }()
	select {
	case msg := <-done:
		if b, ok := msg.(tea.BatchMsg); ok {
			var out []tea.Msg
			for _, c := range b {
				out = append(out, run(c)...)
			}
			return out
		}
		if msg == nil {
			return nil
		}
		return []tea.Msg{msg}
	case <-waitShort():
		return nil
	}
}

func key(s string) tea.KeyMsg {
	switch s {
	case "enter":
		return tea.KeyMsg{Type: tea.KeyEnter}
	case "esc":
		return tea.KeyMsg{Type: tea.KeyEsc}
	case "up":
		return tea.KeyMsg{Type: tea.KeyUp}
	case "down":
		return tea.KeyMsg{Type: tea.KeyDown}
	case "left":
		return tea.KeyMsg{Type: tea.KeyLeft}
	case "right":
		return tea.KeyMsg{Type: tea.KeyRight}
	case "tab":
		return tea.KeyMsg{Type: tea.KeyTab}
	case "space":
		return tea.KeyMsg{Type: tea.KeySpace, Runes: []rune{' '}}
	case "ctrl+p":
		return tea.KeyMsg{Type: tea.KeyCtrlP}
	}
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
}

func checkView(t *testing.T, m *app.Model, w, h int, label string) string {
	t.Helper()
	v := m.View()
	lines := strings.Split(v, "\n")
	if len(lines) > h {
		t.Errorf("%s: %d lines > height %d", label, len(lines), h)
	}
	for i, l := range lines {
		if lw := ansi.StringWidth(l); lw > w {
			t.Errorf("%s: line %d width %d > %d: %q", label, i, lw, w, ansi.Strip(l))
			break
		}
	}
	return v
}

func TestEveryScreenEveryThemeAndSize(t *testing.T) {
	m, ctx := newModel(t)
	sizes := [][2]int{{80, 24}, {120, 40}, {200, 60}, {60, 20}}
	for _, th := range ctx.Themes.Names() {
		ctx.Cfg.Theme = th
		ctx.Apply()
		for _, sz := range sizes {
			drive(m, tea.WindowSizeMsg{Width: sz[0], Height: sz[1]})
			v := checkView(t, m, sz[0], sz[1], th+" dashboard")
			if !strings.Contains(ansi.Strip(v), "TERMVERSE") {
				t.Fatalf("header missing")
			}
			for _, s := range registry.Sections {
				drive(m, ui.NavigateMsg{ID: s.ID})
				checkView(t, m, sz[0], sz[1], th+" "+s.ID)
				drive(m, key("down"))
				drive(m, key("tab"))
				checkView(t, m, sz[0], sz[1], th+" "+s.ID+" after keys")
				drive(m, ui.BackMsg{})
			}
		}
	}
}

func TestMenuNavigationDigitsAndBack(t *testing.T) {
	m, _ := newModel(t)
	drive(m, tea.WindowSizeMsg{Width: 120, Height: 40})
	for d := 0; d <= 9; d++ {
		drive(m, key(string(rune('0'+d))))
		v := ansi.Strip(m.View())
		want := ui.T(registry.Sections[d].Name)
		if !strings.Contains(v, want) {
			t.Errorf("digit %d did not open %s", d, want)
		}
		drive(m, key("esc"))
	}
	// [10] via arrows + Enter: from [0], "up" wraps to the last entry.
	drive(m, key("0"))
	drive(m, key("esc"))
	drive(m, key("up"))
	drive(m, key("enter"))
	if !strings.Contains(ansi.Strip(m.View()), "Settings & About") {
		t.Error("menu 10 not reachable with arrows")
	}
}

func TestHelpAndPalette(t *testing.T) {
	m, ctx := newModel(t)
	drive(m, tea.WindowSizeMsg{Width: 120, Height: 40})
	drive(m, key("?"))
	if !strings.Contains(ansi.Strip(m.View()), "Keyboard shortcuts") {
		t.Error("help overlay missing")
	}
	drive(m, key("x"))
	drive(m, key("ctrl+p"))
	for _, r := range "theme: matrix" {
		drive(m, key(string(r)))
	}
	checkView(t, m, 120, 40, "palette")
	drive(m, key("enter"))
	if ctx.Cfg.Theme != "matrix" {
		t.Errorf("palette theme switch failed: %s", ctx.Cfg.Theme)
	}
}

func TestThemePersistsAfterRestart(t *testing.T) {
	m, ctx := newModel(t)
	drive(m, tea.WindowSizeMsg{Width: 120, Height: 40})
	drive(m, key("8")) // customization, cursor on Theme
	drive(m, key("right"))
	if ctx.Cfg.Theme != "matrix" {
		t.Fatalf("theme = %s", ctx.Cfg.Theme)
	}
	paths, reg, w, _ := app.Setup()
	ctx2, _ := app.Context(paths, reg, w)
	if ctx2.Cfg.Theme != "matrix" {
		t.Errorf("theme not persisted: %s", ctx2.Cfg.Theme)
	}
}

func TestProductivityTaskPersists(t *testing.T) {
	m, ctx := newModel(t)
	drive(m, tea.WindowSizeMsg{Width: 120, Height: 40})
	drive(m, key("6"))
	drive(m, key("a"))
	for _, r := range "ship v1" {
		drive(m, key(string(r)))
	}
	drive(m, key("enter"))
	drive(m, key("space"))
	if !strings.Contains(ansi.Strip(m.View()), "ship v1") {
		t.Fatal("task not shown")
	}
	data := readFile(t, ctx.Paths.DataFile("tasks.json"))
	if !strings.Contains(data, `"ship v1"`) || !strings.Contains(data, `"done": true`) {
		t.Errorf("tasks.json: %s", data)
	}
}

func TestSnakePlayable(t *testing.T) {
	m, _ := newModel(t)
	drive(m, tea.WindowSizeMsg{Width: 120, Height: 40})
	drive(m, key("2"))
	drive(m, key("enter")) // Snake is first
	checkView(t, m, 120, 40, "snake")
	if !strings.Contains(ansi.Strip(m.View()), "Score 0") {
		t.Error("snake not started")
	}
	drive(m, key("p"))
	if !strings.Contains(ansi.Strip(m.View()), "PAUSED") {
		t.Error("pause not shown")
	}
}

func TestSmallTerminal(t *testing.T) {
	m, _ := newModel(t)
	drive(m, tea.WindowSizeMsg{Width: 30, Height: 8})
	if !strings.Contains(m.View(), "too small") {
		t.Error("small terminal message missing")
	}
}

func TestFrenchInterface(t *testing.T) {
	m, ctx := newModel(t)
	ctx.Cfg.Language = "fr"
	ctx.Apply()
	defer ui.SetLanguage("en")
	drive(m, ui.ConfigChangedMsg{})
	drive(m, tea.WindowSizeMsg{Width: 120, Height: 40})
	v := ansi.Strip(m.View())
	if !strings.Contains(v, "Outils développeur") {
		t.Errorf("French menu missing")
	}
}

var _ = system.NA
