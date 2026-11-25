// Package settings implements the "Settings & About" section.
package settings

import (
	"fmt"
	"runtime"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/KAYTAY10ok/TermVerse/internal/config"
	"github.com/KAYTAY10ok/TermVerse/internal/ui"
	"github.com/KAYTAY10ok/TermVerse/internal/version"
)

// Screen is the settings screen.
type Screen struct {
	ctx     *ui.Context
	opts    ui.Options
	confirm ui.Confirm
}

// New creates the screen.
func New(ctx *ui.Context) ui.Screen {
	s := &Screen{ctx: ctx}
	s.build()
	return s
}

func (s *Screen) ID() string      { return "settings" }
func (s *Screen) Title() string   { return ui.T("Settings & About") }
func (s *Screen) Capturing() bool { return false }
func (s *Screen) Init() tea.Cmd   { s.opts.Vim = s.ctx.Vim(); return nil }
func (s *Screen) Keys() []ui.Key {
	return []ui.Key{{Keys: "↑↓", Desc: "select"}, {Keys: "←→", Desc: "change"}, {Keys: "enter", Desc: "action"}, {Keys: "esc", Desc: "back"}}
}

func (s *Screen) save() tea.Cmd { return s.ctx.SaveAndApply("") }

func (s *Screen) build() {
	c := s.ctx.Cfg
	toggle := func(label string, p *bool) ui.Option {
		return ui.Option{Label: label, Value: func() string { return ui.OnOff(*p) },
			Cycle: func(int) tea.Cmd { *p = !*p; return s.save() }}
	}
	intCycle := func(label string, p *int, list []int, unit string) ui.Option {
		return ui.Option{Label: label, Value: func() string { return fmt.Sprintf("%d %s", *p, ui.T(unit)) },
			Cycle: func(d int) tea.Cmd { *p = ui.CycleIndex(list, *p, d); return s.save() }}
	}
	langName := map[string]string{"auto": "Auto ($LANG)", "en": "English", "fr": "Français"}
	s.opts.Items = []ui.Option{
		{Label: "General", Header: true},
		{Label: "Language", Value: func() string { return langName[c.Language] },
			Cycle: func(d int) tea.Cmd { c.Language = ui.CycleIndex(config.Languages, c.Language, d); return s.save() }},
		{Label: "Theme", Value: func() string { return c.Theme },
			Cycle: func(d int) tea.Cmd { c.Theme = ui.CycleIndex(s.ctx.Themes.Names(), c.Theme, d); return s.save() }},
		intCycle("Refresh interval", &c.RefreshSeconds, config.Refreshes, "s"),
		{Label: "More appearance options", Enter: func() tea.Cmd { return ui.Navigate("customize") }},
		{Label: "Navigation", Header: true},
		toggle("Vim keys (h j k l)", &c.Navigation.VimKeys),
		toggle("Mouse support", &c.Navigation.Mouse),
		toggle("Confirm before quitting", &c.Navigation.ConfirmQuit),
		{Label: "Productivity & automation", Header: true},
		intCycle("Pomodoro focus", &c.Productivity.PomodoroWork, []int{15, 20, 25, 30, 45, 50, 60, 90}, "min"),
		intCycle("Pomodoro break", &c.Productivity.PomodoroBreak, []int{3, 5, 10, 15, 20}, "min"),
		intCycle("Command timeout", &c.Automation.TimeoutSeconds, []int{10, 30, 60, 120, 300, 600}, "s"),
		{Label: "Updates", Header: true},
		toggle("Check on startup", &c.Updates.CheckOnStartup),
		{Label: "Check for updates now", Enter: func() tea.Cmd { return ui.Navigate("updates.check") }},
		{Label: "Maintenance", Header: true},
		{Label: "Reset configuration", Enter: func() tea.Cmd {
			s.confirm.Ask(ui.T("Reset config.json to defaults? Themes, notes, tasks and scores are kept."), func() tea.Cmd {
				fresh, err := config.Reset(s.ctx.Paths)
				if err != nil {
					return ui.Notify(ui.Error, err.Error())
				}
				*s.ctx.Cfg = *fresh // keep the pointer shared with every screen
				s.ctx.Apply()
				return ui.Batch(ui.ConfigChanged, ui.Notify(ui.Success, ui.T("Configuration reset to defaults")))
			})
			return nil
		}},
	}
	s.opts.Fix()
}

func (s *Screen) Update(msg tea.Msg) (ui.Screen, tea.Cmd) {
	km, ok := msg.(tea.KeyMsg)
	if !ok {
		return s, nil
	}
	if s.confirm.Active {
		return s, s.confirm.Update(msg)
	}
	if km.String() == "esc" || km.String() == "q" {
		return s, ui.BackCmd
	}
	_, cmd := s.opts.HandleKey(km)
	return s, cmd
}

func (s *Screen) View(w, h int) string {
	st := s.ctx.S
	leftW := w
	wide := w >= 96
	if wide {
		leftW = w / 2
	}
	iw, ih := st.PanelInner(leftW, h)
	bottom := st.Muted.Render(ui.T("Settings are saved immediately."))
	if s.confirm.Active {
		bottom = s.confirm.View(st)
	}
	bh := strings.Count(bottom, "\n") + 1
	body := ui.Fit(s.opts.View(st, iw, ih-bh-1), iw, ih-bh-1) + "\n" + st.Sep(iw) + "\n" + bottom
	left := st.Panel(ui.T("Settings"), body, leftW, h, true)
	if !wide {
		return left
	}
	rw := w - leftW - 1
	return lipgloss.JoinHorizontal(lipgloss.Top, left, " ", st.Panel(ui.T("About"), s.about(rw-4), rw, h, false))
}

func (s *Screen) about(width int) string {
	st := s.ctx.S
	rows := [][2]string{
		{ui.T("Version"), "v" + version.Version},
		{ui.T("Commit"), version.Commit},
		{ui.T("Built"), version.Date},
		{ui.T("Go"), runtime.Version()},
		{ui.T("Platform"), runtime.GOOS + "/" + runtime.GOARCH},
		{ui.T("Config"), s.ctx.Paths.ConfigFile},
		{ui.T("Themes"), s.ctx.Paths.ThemesDir},
		{ui.T("Data"), s.ctx.Paths.DataDir},
		{ui.T("GitHub"), version.RepoURL()},
		{ui.T("License"), version.License},
	}
	out := st.Title.Render("TermVerse") + "\n" +
		st.Text.Render(ui.Wrap(ui.T("A modern, customizable and modular terminal environment: dashboard, developer tools, arcade, system and network utilities, productivity and automation."), width)) +
		"\n\n" + st.KV(rows, 10)
	if len(s.ctx.Warnings) > 0 {
		out += "\n\n" + st.Warning.Render(ui.T("Warnings at startup")) + "\n"
		for _, w := range s.ctx.Warnings {
			out += st.Muted.Render(ui.Wrap("• "+w, width)) + "\n"
		}
	}
	out += "\n" + st.Muted.Render("MIT License · © 2026 TermVerse contributors")
	return out
}
