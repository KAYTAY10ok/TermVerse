package app

import (
	"sort"
	"strings"
	"unicode"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/KAYTAY10ok/TermVerse/internal/registry"
	"github.com/KAYTAY10ok/TermVerse/internal/ui"
)

// paletteEntry is an action of the command palette.
type paletteEntry struct {
	label string
	hint  string
	run   func(m *Model) tea.Cmd
}

type palette struct {
	input   textinput.Model
	list    ui.List
	entries []paletteEntry
}

func newPalette(m *Model) *palette {
	p := &palette{input: m.ctx.S.NewInput("Type a command…", 80)}
	p.input.Focus()
	p.entries = buildEntries(m)
	p.filter()
	return p
}

func buildEntries(m *Model) []paletteEntry {
	var es []paletteEntry
	nav := func(id string) func(*Model) tea.Cmd {
		return func(*Model) tea.Cmd { return ui.Navigate(id) }
	}
	es = append(es, paletteEntry{label: ui.T("Home"), hint: ui.T("Dashboard"), run: func(m *Model) tea.Cmd { m.stack = nil; return nil }})
	for _, s := range registry.Sections {
		es = append(es, paletteEntry{label: dashLabel(s), hint: ui.T(s.Description), run: nav(s.ID)})
	}
	for _, c := range registry.Components {
		parent, _ := registry.Section(c.Parent)
		es = append(es, paletteEntry{label: ui.T(parent.Name) + " › " + ui.T(c.Name), hint: ui.T(c.Description), run: nav(c.ID)})
	}
	for _, n := range m.ctx.Themes.Names() {
		name := n
		es = append(es, paletteEntry{label: ui.T("Theme: ") + name, hint: ui.T("Switch theme live"), run: func(m *Model) tea.Cmd {
			m.ctx.Cfg.Theme = name
			return m.ctx.SaveAndApply("Theme applied")
		}})
	}
	for _, l := range []struct{ code, name string }{{"en", "English"}, {"fr", "Français"}, {"auto", "Auto"}} {
		code := l.code
		es = append(es, paletteEntry{label: ui.T("Language: ") + l.name, hint: ui.T("Interface language"), run: func(m *Model) tea.Cmd {
			m.ctx.Cfg.Language = code
			cmd := m.ctx.SaveAndApply("Language changed")
			m.rebuildMenu()
			return cmd
		}})
	}
	es = append(es,
		paletteEntry{label: ui.T("Toggle borders"), hint: ui.T("Customization"), run: func(m *Model) tea.Cmd {
			m.ctx.Cfg.Appearance.Borders = !m.ctx.Cfg.Appearance.Borders
			return m.ctx.SaveAndApply("Saved")
		}},
		paletteEntry{label: ui.T("Toggle animations"), hint: ui.T("Customization"), run: func(m *Model) tea.Cmd {
			m.ctx.Cfg.Appearance.Animations = !m.ctx.Cfg.Appearance.Animations
			return m.ctx.SaveAndApply("Saved")
		}},
		paletteEntry{label: ui.T("Keyboard shortcuts"), hint: "?", run: func(m *Model) tea.Cmd { m.showHelp = true; return nil }},
		paletteEntry{label: ui.T("Quit TermVerse"), hint: "q", run: func(m *Model) tea.Cmd { return tea.Quit }},
	)
	return es
}

// fuzzyScore returns a match score of query in s (higher is better), or -1.
func fuzzyScore(query, s string) int {
	q := []rune(strings.ToLower(query))
	if len(q) == 0 {
		return 0
	}
	t := []rune(strings.ToLower(s))
	score, qi, streak := 0, 0, 0
	for i, r := range t {
		if qi < len(q) && r == q[qi] {
			qi++
			streak++
			score += 1 + streak*2
			if i == 0 || !unicode.IsLetter(t[i-1]) {
				score += 5 // word start bonus
			}
		} else {
			streak = 0
		}
	}
	if qi < len(q) {
		return -1
	}
	if strings.Contains(string(t), string(q)) {
		score += 20
	}
	return score - len(t)/10
}

func (p *palette) filter() {
	q := strings.TrimSpace(p.input.Value())
	type scored struct {
		i, s int
	}
	var res []scored
	for i, e := range p.entries {
		sc := fuzzyScore(q, e.label+" "+e.hint)
		if sc >= 0 {
			res = append(res, scored{i, sc})
		}
	}
	if q != "" {
		sort.SliceStable(res, func(a, b int) bool { return res[a].s > res[b].s })
	}
	items := make([]ui.Item, 0, len(res))
	for _, r := range res {
		e := p.entries[r.i]
		items = append(items, ui.Item{Title: e.label, Right: e.hint, Value: r.i})
	}
	p.list.Cursor = 0
	p.list.SetItems(items)
}

// update returns (closePalette, cmd).
func (p *palette) update(m *Model, msg tea.Msg) (bool, tea.Cmd) {
	if km, ok := msg.(tea.KeyMsg); ok {
		switch km.String() {
		case "esc", "ctrl+p":
			return true, nil
		case "enter":
			if it, ok := p.list.Selected(); ok {
				return true, p.entries[it.Value.(int)].run(m)
			}
			return true, nil
		case "up", "down", "pgup", "pgdown":
			p.list.HandleKey(km)
			return false, nil
		}
	}
	var cmd tea.Cmd
	before := p.input.Value()
	p.input, cmd = p.input.Update(msg)
	if p.input.Value() != before {
		p.filter()
	}
	return false, cmd
}

func (p *palette) view(s ui.Styles, width, height int) string {
	w := min(72, width-4)
	h := min(20, height-2)
	iw, ih := s.PanelInner(w, h)
	p.input.Width = iw - 4
	body := p.input.View() + "\n" + s.Sep(iw) + "\n" + p.list.View(s, iw, ih-2)
	return s.Panel(ui.T("Command palette"), body, w, h, true)
}
