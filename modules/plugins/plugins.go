// Package plugins implements the "Plugin Hub": it lists built-in modules,
// lets the user enable/disable optional ones and shows local plugin
// manifests. It never downloads or runs plugin code.
package plugins

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/KAYTAY10ok/TermVerse/internal/registry"
	"github.com/KAYTAY10ok/TermVerse/internal/ui"
)

type entryKind int

const (
	kindSection entryKind = iota
	kindComponent
	kindLocal
)

type entry struct {
	kind entryKind
	mod  registry.Module
	man  registry.Manifest
}

var tabs = []string{"Sections", "Components", "Local plugins"}

// Screen is the Plugin Hub.
type Screen struct {
	ctx    *ui.Context
	tab    int
	list   ui.List
	local  []registry.Manifest
	locErr error
}

// New creates the screen.
func New(ctx *ui.Context) ui.Screen { return &Screen{ctx: ctx} }

func (s *Screen) ID() string      { return "plugins" }
func (s *Screen) Title() string   { return ui.T("Plugin Hub") }
func (s *Screen) Capturing() bool { return false }

func (s *Screen) Init() tea.Cmd {
	s.local, s.locErr = registry.Discover(s.ctx.Paths.PluginsDir)
	s.list.Vim = s.ctx.Vim()
	s.refresh()
	return nil
}

func (s *Screen) Keys() []ui.Key {
	k := []ui.Key{{Keys: "tab", Desc: "switch tab"}, {Keys: "↑↓", Desc: "select"}}
	if s.tab == 0 {
		k = append(k, ui.Key{Keys: "space", Desc: "enable/disable"}, ui.Key{Keys: "enter", Desc: "open"})
	}
	if s.tab == 2 {
		k = append(k, ui.Key{Keys: "r", Desc: "rescan"})
	}
	return append(k, ui.Key{Keys: "esc", Desc: "back"})
}

func (s *Screen) status(m registry.Module) string {
	if !m.Optional {
		return ui.T("core")
	}
	if s.ctx.Cfg.ModuleEnabled(m.ID) {
		return ui.T("enabled")
	}
	return ui.T("disabled")
}

func (s *Screen) refresh() {
	var items []ui.Item
	switch s.tab {
	case 0:
		for _, m := range registry.Sections {
			items = append(items, ui.Item{Title: fmt.Sprintf("[%d] %s", m.Number, ui.T(m.Name)), Right: s.status(m),
				Disabled: m.Optional && !s.ctx.Cfg.ModuleEnabled(m.ID), Value: entry{kind: kindSection, mod: m}})
		}
	case 1:
		for _, m := range registry.Components {
			p, _ := registry.Section(m.Parent)
			items = append(items, ui.Item{Title: ui.T(m.Name), Right: ui.T(p.Name), Value: entry{kind: kindComponent, mod: m}})
		}
	case 2:
		for _, m := range s.local {
			r := m.Version
			if m.Error != "" {
				r = ui.T("invalid")
			}
			items = append(items, ui.Item{Title: ui.Sanitize(m.Name), Right: ui.Sanitize(r), Value: entry{kind: kindLocal, man: m}})
		}
	}
	s.list.SetItems(items)
}

func (s *Screen) Update(msg tea.Msg) (ui.Screen, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.MouseMsg:
		s.list.HandleMouse(msg)
	case tea.KeyMsg:
		switch msg.String() {
		case "esc", "q":
			return s, ui.BackCmd
		case "tab", "right", "l":
			s.tab = (s.tab + 1) % len(tabs)
			s.list.Cursor = 0
			s.refresh()
		case "shift+tab", "left", "h":
			s.tab = (s.tab + len(tabs) - 1) % len(tabs)
			s.list.Cursor = 0
			s.refresh()
		case "r":
			s.local, s.locErr = registry.Discover(s.ctx.Paths.PluginsDir)
			s.refresh()
		case " ", "e":
			if it, ok := s.list.Selected(); ok && s.tab == 0 {
				m := it.Value.(entry).mod
				if !m.Optional {
					return s, ui.Notify(ui.Warn, ui.Tf("%s is a core module and cannot be disabled", ui.T(m.Name)))
				}
				on := !s.ctx.Cfg.ModuleEnabled(m.ID)
				s.ctx.Cfg.Modules[m.ID] = on
				s.refresh()
				state := "disabled"
				if on {
					state = "enabled"
				}
				return s, s.ctx.SaveAndApply(ui.T(m.Name) + " " + ui.T(state))
			}
		case "enter":
			if it, ok := s.list.Selected(); ok {
				e := it.Value.(entry)
				switch e.kind {
				case kindSection:
					return s, ui.Navigate(e.mod.ID)
				case kindComponent:
					return s, ui.Navigate(e.mod.ID)
				}
			}
		default:
			s.list.HandleKey(msg)
		}
	}
	return s, nil
}

func (s *Screen) View(w, h int) string {
	st := s.ctx.S
	wide := w >= 90
	leftW := w
	if wide {
		leftW = w / 2
	}
	iw, ih := st.PanelInner(leftW, h)
	body := st.Tabs(tabs, s.tab) + "\n" + st.Sep(iw) + "\n"
	if s.tab == 2 && len(s.local) == 0 {
		body += st.Muted.Render(ui.Wrap(ui.Tf("No local plugins found in %s", s.ctx.Paths.PluginsDir), iw))
		if s.locErr != nil {
			body += "\n" + st.Error.Render(s.locErr.Error())
		}
	} else {
		body += s.list.View(st, iw, ih-2)
	}
	left := st.Panel(ui.Tf("Plugin Hub — %d built-in modules", registry.Count()), body, leftW, h, true)
	if !wide {
		return left
	}
	rw := w - leftW - 1
	return lipgloss.JoinHorizontal(lipgloss.Top, left, " ", st.Panel(ui.T("Details"), s.details(rw-4), rw, h, false))
}

func (s *Screen) details(width int) string {
	st := s.ctx.S
	it, ok := s.list.Selected()
	if !ok {
		return s.about(width)
	}
	e := it.Value.(entry)
	var b strings.Builder
	switch e.kind {
	case kindSection, kindComponent:
		m := e.mod
		b.WriteString(st.Title.Render(ui.T(m.Name)) + "\n\n")
		rows := [][2]string{{ui.T("ID"), m.ID}, {ui.T("Version"), m.Version}, {ui.T("Type"), ui.T("built-in")}}
		if e.kind == kindSection {
			rows = append(rows, [2]string{ui.T("Status"), s.status(m)})
		} else {
			p, _ := registry.Section(m.Parent)
			rows = append(rows, [2]string{ui.T("Section"), ui.T(p.Name)})
		}
		b.WriteString(st.KV(rows, 10) + "\n\n" + st.Text.Render(ui.Wrap(ui.T(m.Description), width)))
	case kindLocal:
		m := e.man
		b.WriteString(st.Title.Render(ui.Sanitize(m.Name)) + "\n\n")
		b.WriteString(st.KV([][2]string{
			{ui.T("Version"), ui.Sanitize(m.Version)}, {ui.T("Author"), ui.Sanitize(m.Author)},
			{ui.T("Homepage"), ui.Sanitize(m.Homepage)}, {ui.T("Folder"), m.Dir},
		}, 10) + "\n\n")
		if m.Error != "" {
			b.WriteString(st.Error.Render(m.Error) + "\n\n")
		}
		b.WriteString(st.Text.Render(ui.Wrap(ui.Sanitize(m.Description), width)) + "\n\n")
		b.WriteString(st.Warning.Render(ui.Wrap(ui.T("Community plugin execution is planned for a future version. TermVerse only reads manifests and never runs plugin code."), width)))
	}
	b.WriteString("\n\n" + s.about(width))
	return b.String()
}

func (s *Screen) about(width int) string {
	st := s.ctx.S
	return st.Muted.Render(ui.Wrap(ui.Tf("Local plugins live in %s/<name>/plugin.json (fields: name, version, description, author, homepage).", s.ctx.Paths.PluginsDir), width))
}
