// Package updates implements the "Updates & News" section.
package updates

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/KAYTAY10ok/TermVerse/internal/ui"
	"github.com/KAYTAY10ok/TermVerse/internal/update"
	"github.com/KAYTAY10ok/TermVerse/internal/version"
)

var tabs = []string{"What's new", "Changelog", "Release check"}

type checkMsg struct {
	rel update.Release
	err error
}

// Screen is the updates screen.
type Screen struct {
	ctx      *ui.Context
	tab      int
	scroll   ui.Scroller
	checking bool
	checked  bool
	rel      update.Release
	err      error
}

// New creates the screen.
func New(ctx *ui.Context) ui.Screen { return &Screen{ctx: ctx} }

func (s *Screen) ID() string        { return "updates" }
func (s *Screen) Title() string     { return ui.T("Updates & News") }
func (s *Screen) Capturing() bool   { return false }
func (s *Screen) Init() tea.Cmd     { s.scroll.Vim = s.ctx.Vim(); return nil }
func (s *Screen) statePath() string { return s.ctx.Paths.DataFile("state.json") }

// OpenSub opens a tab by name ("check").
func (s *Screen) OpenSub(sub string) tea.Cmd {
	if sub == "check" {
		s.tab = 2
		return s.check()
	}
	return nil
}

func (s *Screen) Keys() []ui.Key {
	k := []ui.Key{{Keys: "tab", Desc: "switch tab"}, {Keys: "↑↓", Desc: "scroll"}}
	if s.tab == 2 {
		k = append(k, ui.Key{Keys: "c", Desc: "check now"})
	}
	return append(k, ui.Key{Keys: "esc", Desc: "back"})
}

func (s *Screen) check() tea.Cmd {
	if s.checking {
		return nil
	}
	s.checking = true
	repo := s.ctx.Cfg.Updates.Repository
	return func() tea.Msg {
		r, err := update.Latest(context.Background(), repo)
		return checkMsg{r, err}
	}
}

func (s *Screen) Update(msg tea.Msg) (ui.Screen, tea.Cmd) {
	switch msg := msg.(type) {
	case checkMsg:
		s.checking, s.checked = false, true
		s.rel, s.err = msg.rel, msg.err
		now := time.Now()
		st := update.State{LastCheck: now}
		if msg.err == nil {
			st.Latest = msg.rel.Version()
		}
		_ = update.SaveState(s.statePath(), st)
		s.ctx.LastUpdateCheck, s.ctx.LatestVersion = now, st.Latest
		if msg.err == nil && update.Compare(version.Version, msg.rel.Version()) < 0 {
			return s, ui.Notify(ui.Info, ui.Tf("TermVerse %s is available", msg.rel.Tag))
		}
	case tea.KeyMsg:
		switch msg.String() {
		case "esc", "q":
			return s, ui.BackCmd
		case "tab", "right", "l":
			s.tab = (s.tab + 1) % len(tabs)
			s.scroll.Top()
		case "shift+tab", "left", "h":
			s.tab = (s.tab + len(tabs) - 1) % len(tabs)
			s.scroll.Top()
		case "c", "r":
			if s.tab == 2 {
				return s, s.check()
			}
		default:
			s.scroll.HandleKey(msg)
		}
	case tea.MouseMsg:
		s.scroll.HandleMouse(msg)
	}
	return s, nil
}

func (s *Screen) View(w, h int) string {
	st := s.ctx.S
	iw, ih := st.PanelInner(w, h)
	var content string
	switch s.tab {
	case 0:
		sec := update.ChangelogSection(s.ctx.Changelog, version.Version)
		if sec == "" {
			sec = ui.T("No release notes found for this version.")
		}
		head := st.Title.Render(ui.Tf("You are running TermVerse v%s", version.Version)) + "\n" +
			st.Muted.Render(ui.Tf("Commit %s · built %s", version.Commit, version.Date)) + "\n\n"
		content = head + st.Markdown(sec, iw)
	case 1:
		content = st.Markdown(s.ctx.Changelog, iw)
	case 2:
		content = s.checkView(iw)
	}
	s.scroll.SetContent(content)
	body := st.Tabs(tabs, s.tab) + "\n" + st.Sep(iw) + "\n" + s.scroll.View(st, iw, ih-2)
	return st.Panel(s.Title(), body, w, h, true)
}

func (s *Screen) checkView(width int) string {
	st := s.ctx.S
	var b strings.Builder
	b.WriteString(st.KV([][2]string{
		{ui.T("Installed"), "v" + version.Version},
		{ui.T("Repository"), "github.com/" + s.ctx.Cfg.Updates.Repository},
	}, 12) + "\n\n")
	switch {
	case s.checking:
		b.WriteString(st.Secondary.Render(ui.T("Contacting GitHub… (timeout 10s)")))
	case !s.checked:
		b.WriteString(st.Text.Render(ui.T("Press c to check GitHub for the latest release.")) + "\n")
		b.WriteString(st.Muted.Render(ui.T("TermVerse never downloads or installs anything without your explicit action.")))
	case errors.Is(s.err, update.ErrNoRelease):
		b.WriteString(st.Warning.Render(ui.T("No release has been published on GitHub yet.")))
	case s.err != nil:
		b.WriteString(st.Error.Render(ui.T("Update check failed: ") + s.err.Error()))
	default:
		latest := s.rel.Version()
		b.WriteString(st.KV([][2]string{
			{ui.T("Latest"), s.rel.Tag},
			{ui.T("Published"), s.rel.PublishedAt.Local().Format("2006-01-02")},
			{ui.T("Page"), s.rel.URL},
		}, 12) + "\n\n")
		if update.Compare(version.Version, latest) >= 0 {
			b.WriteString(st.Success.Render("✔ " + ui.T("You are up to date.")))
		} else {
			b.WriteString(st.Warning.Render("⬆ "+ui.Tf("Version %s is available.", latest)) + "\n\n")
			b.WriteString(st.Text.Render(ui.T("To update, run this command yourself:")) + "\n")
			b.WriteString(st.Accent.Render(fmt.Sprintf("  curl -fsSL https://raw.githubusercontent.com/%s/main/scripts/install.sh | bash", s.ctx.Cfg.Updates.Repository)) + "\n\n")
			if body := strings.TrimSpace(ui.Sanitize(s.rel.Body)); body != "" {
				b.WriteString(st.Title.Render(ui.T("Release notes")) + "\n" + st.Markdown(body, width))
			}
		}
	}
	if !s.ctx.LastUpdateCheck.IsZero() {
		b.WriteString("\n\n" + st.Muted.Render(ui.T("Last check: ")+s.ctx.LastUpdateCheck.Local().Format("2006-01-02 15:04:05")))
	}
	return b.String()
}
