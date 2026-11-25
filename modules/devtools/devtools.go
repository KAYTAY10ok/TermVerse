// Package devtools implements the "Developer Tools" section.
package devtools

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/KAYTAY10ok/TermVerse/internal/ui"
)

// tool is one developer utility.
type tool interface {
	id() string
	name() string
	desc() string
	open() tea.Cmd
	update(msg tea.Msg) tea.Cmd
	view(w, h int) string
	keys() []ui.Key
	capturing() bool
}

// Screen is the developer tools hub.
type Screen struct {
	ctx    *ui.Context
	tools  []tool
	list   ui.List
	active tool
}

// New creates the screen.
func New(ctx *ui.Context) ui.Screen {
	s := &Screen{ctx: ctx}
	s.tools = []tool{
		newGitTool(ctx), newJSONTool(ctx), newUUIDTool(ctx), newBase64Tool(ctx), newHashTool(ctx),
		newTimeTool(ctx), newPasswordTool(ctx), newRegexTool(ctx), newHTTPTool(ctx), newPortsTool(ctx),
	}
	return s
}

func (s *Screen) ID() string    { return "devtools" }
func (s *Screen) Title() string { return ui.T("Developer Tools") }
func (s *Screen) Capturing() bool {
	return s.active != nil && s.active.capturing()
}

func (s *Screen) Init() tea.Cmd {
	s.list.Vim = s.ctx.Vim()
	items := make([]ui.Item, len(s.tools))
	for i, t := range s.tools {
		items[i] = ui.Item{Title: ui.T(t.name()), Desc: ui.T(t.desc()), Value: i}
	}
	s.list.SetItems(items)
	return nil
}

// OpenSub opens a tool by id (e.g. "uuid").
func (s *Screen) OpenSub(sub string) tea.Cmd {
	for i, t := range s.tools {
		if t.id() == sub {
			s.list.Cursor = i
			s.active = t
			return t.open()
		}
	}
	return nil
}

func (s *Screen) Keys() []ui.Key {
	if s.active != nil {
		return append(s.active.keys(), ui.Key{Keys: "esc", Desc: "tools"})
	}
	return []ui.Key{{Keys: "↑↓", Desc: "select"}, {Keys: "enter", Desc: "open"}, {Keys: "esc", Desc: "back"}}
}

func (s *Screen) Update(msg tea.Msg) (ui.Screen, tea.Cmd) {
	if km, ok := msg.(tea.KeyMsg); ok {
		if s.active != nil {
			if km.String() == "esc" {
				s.active = nil
				return s, nil
			}
			return s, s.active.update(msg)
		}
		switch km.String() {
		case "esc", "q":
			return s, ui.BackCmd
		case "enter", "right", "l":
			if it, ok := s.list.Selected(); ok {
				s.active = s.tools[it.Value.(int)]
				return s, s.active.open()
			}
		default:
			s.list.HandleKey(km)
		}
		return s, nil
	}
	if mm, ok := msg.(tea.MouseMsg); ok && s.active == nil {
		s.list.HandleMouse(mm)
		return s, nil
	}
	// Async results go to every tool (each ignores foreign messages).
	var cmds []tea.Cmd
	for _, t := range s.tools {
		cmds = append(cmds, t.update(msg))
	}
	return s, tea.Batch(cmds...)
}

func (s *Screen) View(w, h int) string {
	st := s.ctx.S
	if s.active != nil {
		iw, ih := st.PanelInner(w, h)
		return st.Panel(s.Title()+" › "+ui.T(s.active.name()), s.active.view(iw, ih), w, h, true)
	}
	wide := w >= 90
	leftW := w
	if wide {
		leftW = w / 2
	}
	iw, ih := st.PanelInner(leftW, h)
	left := st.Panel(s.Title(), s.list.View(st, iw, ih), leftW, h, true)
	if !wide {
		return left
	}
	rw := w - leftW - 1
	return lipgloss.JoinHorizontal(lipgloss.Top, left, " ", st.Panel(ui.T("About"), s.about(rw-4), rw, h, false))
}

func (s *Screen) about(w int) string {
	st := s.ctx.S
	it, ok := s.list.Selected()
	if !ok {
		return ""
	}
	t := s.tools[it.Value.(int)]
	out := st.Title.Render(ui.T(t.name())) + "\n\n" + st.Text.Render(ui.Wrap(ui.T(t.desc()), w)) + "\n\n" + st.Title.Render(ui.T("Keys")) + "\n"
	for _, k := range t.keys() {
		out += st.Key.Render(ui.PadRight(k.Keys, 12)) + st.Muted.Render(ui.T(k.Desc)) + "\n"
	}
	return out + "\n" + st.Muted.Render(ui.Wrap(ui.T("More tools (Docker, API collections, …) are planned; each tool is a self-contained component."), w))
}
