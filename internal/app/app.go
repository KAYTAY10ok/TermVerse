// Package app wires the TermVerse TUI together: the dashboard, navigation
// between screens, overlays (help, palette) and background refreshes.
package app

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

	"github.com/KAYTAY10ok/TermVerse/internal/dashboard"
	"github.com/KAYTAY10ok/TermVerse/internal/registry"
	"github.com/KAYTAY10ok/TermVerse/internal/system"
	"github.com/KAYTAY10ok/TermVerse/internal/ui"
	"github.com/KAYTAY10ok/TermVerse/internal/version"
)

// Factory builds a screen.
type Factory func(ctx *ui.Context) ui.Screen

type clockMsg time.Time
type sysTickMsg struct{}
type sysMsg system.Snapshot

// Model is the root Bubble Tea model.
type Model struct {
	ctx       *ui.Context
	factories map[string]Factory
	screens   map[string]ui.Screen

	stack    []string // open screens, empty = dashboard
	menu     ui.List
	layout   dashboard.Layout
	showHelp bool
	pal      *palette
	quitAsk  bool

	width, height int
	now           time.Time
	collecting    bool

	notice   ui.NotifyMsg
	noticeAt time.Time
}

// New creates the root model.
func New(ctx *ui.Context, factories map[string]Factory) *Model {
	m := &Model{ctx: ctx, factories: factories, screens: map[string]ui.Screen{}, now: time.Now()}
	m.rebuildMenu()
	if len(ctx.Warnings) > 0 {
		m.notice = ui.NotifyMsg{Level: ui.Warn, Text: ctx.Warnings[0]}
		m.noticeAt = time.Now()
	}
	return m
}

func dashLabel(s registry.Module) string { return dashboard.MenuLabel(s) }

func (m *Model) rebuildMenu() {
	items := make([]ui.Item, 0, len(registry.Sections))
	for _, s := range registry.Sections {
		it := ui.Item{Title: dashLabel(s), Value: s}
		if s.Optional && !m.ctx.Cfg.ModuleEnabled(s.ID) {
			it.Disabled = true
			it.Right = ui.T("off")
		}
		items = append(items, it)
	}
	m.menu.SetItems(items)
	m.menu.Vim = m.ctx.Vim()
}

// Init starts the clock and the system refresh loop.
func (m *Model) Init() tea.Cmd {
	return tea.Batch(clockTick(), m.collect(), tea.SetWindowTitle("TermVerse"))
}

func clockTick() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg { return clockMsg(t) })
}

func (m *Model) active() ui.Screen {
	if len(m.stack) == 0 {
		return nil
	}
	return m.screens[m.stack[len(m.stack)-1]]
}

// collect gathers a snapshot in a goroutine (tea.Cmd), never blocking the UI.
func (m *Model) collect() tea.Cmd {
	if m.collecting {
		return nil
	}
	m.collecting = true
	detailed := false
	if a := m.active(); a != nil && a.ID() == "system" {
		detailed = true
	}
	c := m.ctx.Collector
	return func() tea.Msg { return sysMsg(c.Collect(detailed)) }
}

func (m *Model) open(id, sub string) tea.Cmd {
	sec, ok := registry.Section(id)
	if !ok {
		return nil
	}
	if sec.Optional && !m.ctx.Cfg.ModuleEnabled(id) {
		return ui.Notify(ui.Warn, ui.Tf("%s is disabled — enable it in the Plugin Hub [9]", ui.T(sec.Name)))
	}
	f, ok := m.factories[id]
	if !ok {
		return ui.Notify(ui.Error, "unknown screen "+id)
	}
	if a := m.active(); a != nil {
		if l, ok := a.(ui.Leaver); ok {
			l.OnLeave()
		}
		if a.ID() == id {
			m.stack = m.stack[:len(m.stack)-1]
		}
	}
	scr, ok := m.screens[id]
	if !ok {
		scr = f(m.ctx)
		m.screens[id] = scr
	}
	// Avoid duplicates in the stack.
	for i, s := range m.stack {
		if s == id {
			m.stack = append(m.stack[:i], m.stack[i+1:]...)
			break
		}
	}
	m.stack = append(m.stack, id)
	cmds := []tea.Cmd{scr.Init()}
	if sub != "" {
		if dl, ok := scr.(ui.DeepLinker); ok {
			cmds = append(cmds, dl.OpenSub(sub))
		}
	}
	return tea.Batch(cmds...)
}

func (m *Model) back() {
	if a := m.active(); a != nil {
		if l, ok := a.(ui.Leaver); ok {
			l.OnLeave()
		}
		m.stack = m.stack[:len(m.stack)-1]
	}
}

// Update handles messages.
func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
	case clockMsg:
		m.now = time.Time(msg)
		if !m.noticeAt.IsZero() && m.now.Sub(m.noticeAt) > 6*time.Second {
			m.notice, m.noticeAt = ui.NotifyMsg{}, time.Time{}
		}
		cmds = append(cmds, clockTick())
	case sysTickMsg:
		return m, m.collect()
	case sysMsg:
		m.collecting = false
		m.ctx.Sys, m.ctx.SysOK = system.Snapshot(msg), true
		interval := time.Duration(m.ctx.Cfg.RefreshSeconds) * time.Second
		cmds = append(cmds, tea.Tick(interval, func(time.Time) tea.Msg { return sysTickMsg{} }))
	case ui.NotifyMsg:
		m.notice, m.noticeAt = msg, time.Now()
		return m, nil
	case ui.BackMsg:
		m.back()
		return m, nil
	case ui.NavigateMsg:
		return m, m.open(msg.ID, msg.Sub)
	case ui.ConfigChangedMsg:
		m.rebuildMenu()
		mouse := tea.DisableMouse
		if m.ctx.Cfg.Navigation.Mouse {
			mouse = tea.EnableMouseCellMotion
		}
		cmds = append(cmds, mouse)
	case tea.KeyMsg:
		return m, m.handleKey(msg)
	case tea.MouseMsg:
		return m, m.handleMouse(msg)
	}
	// Broadcast other messages to every live screen (timers keep running in
	// background screens; each screen ignores messages it does not own).
	for id, s := range m.screens {
		ns, cmd := s.Update(msg)
		m.screens[id] = ns
		cmds = append(cmds, cmd)
	}
	return m, tea.Batch(cmds...)
}

func (m *Model) handleKey(msg tea.KeyMsg) tea.Cmd {
	k := msg.String()
	if k == "ctrl+c" {
		return tea.Quit
	}
	if m.pal != nil {
		closeIt, cmd := m.pal.update(m, msg)
		if closeIt {
			m.pal = nil
		}
		return cmd
	}
	if m.showHelp {
		m.showHelp = false
		return nil
	}
	if m.quitAsk {
		m.quitAsk = false
		if k == "y" || k == "Y" || k == "o" || k == "O" || k == "enter" {
			return tea.Quit
		}
		return nil
	}
	a := m.active()
	capturing := a != nil && a.Capturing()
	if !capturing {
		switch k {
		case "ctrl+p":
			m.pal = newPalette(m)
			return textinputBlink()
		case "?":
			m.showHelp = true
			return nil
		}
	}
	if a != nil {
		ns, cmd := a.Update(msg)
		m.screens[a.ID()] = ns
		return cmd
	}
	// Dashboard keys.
	switch {
	case k == "q":
		if m.ctx.Cfg.Navigation.ConfirmQuit {
			m.quitAsk = true
			return nil
		}
		return tea.Quit
	case k == "enter" || k == "right" || (m.ctx.Vim() && k == "l"):
		if it, ok := m.menu.Selected(); ok {
			return m.open(it.Value.(registry.Module).ID, "")
		}
	case len(k) == 1 && k[0] >= '0' && k[0] <= '9':
		n := int(k[0] - '0')
		m.menu.Cursor = n
		return m.open(registry.Sections[n].ID, "")
	default:
		m.menu.HandleKey(msg)
	}
	return nil
}

func (m *Model) handleMouse(msg tea.MouseMsg) tea.Cmd {
	if !m.ctx.Cfg.Navigation.Mouse || m.pal != nil || m.showHelp {
		return nil
	}
	if a := m.active(); a != nil {
		ns, cmd := a.Update(msg)
		m.screens[a.ID()] = ns
		return cmd
	}
	if m.menu.HandleMouse(msg) {
		return nil
	}
	if msg.Action == tea.MouseActionPress && msg.Button == tea.MouseButtonLeft {
		// header occupies one line above the body
		row := msg.Y - 1 - m.layout.MenuY
		col := msg.X - m.layout.MenuX
		if row >= 0 && row < m.layout.MenuRows && row < len(m.menu.Items) && col >= 0 && col < m.layout.MenuW {
			m.menu.Cursor = row
			return m.open(registry.Sections[row].ID, "")
		}
	}
	return nil
}

var weekdays = []string{"Sunday", "Monday", "Tuesday", "Wednesday", "Thursday", "Friday", "Saturday"}
var months = []string{"January", "February", "March", "April", "May", "June", "July", "August", "September", "October", "November", "December"}

func (m *Model) clock() string {
	t := m.now
	return fmt.Sprintf("%s %d %s %d  %s", ui.T(weekdays[t.Weekday()])[:3], t.Day(),
		ui.T(months[t.Month()-1])[:3], t.Year(), t.Format("15:04:05"))
}

func (m *Model) header() string {
	s := m.ctx.S
	logo := ui.Gradient("◆ TERMVERSE", s.C.Primary, s.C.Secondary, 0)
	status := s.Success.Render("●") + " " + s.Muted.Render(ui.T("ready"))
	if !m.ctx.SysOK {
		status = s.Warning.Render("●") + " " + s.Muted.Render(ui.T("starting"))
	}
	left := logo + " " + s.Muted.Render("v"+version.Version) + "   " + status
	if a := m.active(); a != nil {
		sec, _ := registry.Section(a.ID())
		left += s.Muted.Render("   › ") + s.Accent.Render(fmt.Sprintf("[%d] ", sec.Number)) + s.Bold.Render(a.Title())
	}
	right := ""
	if m.ctx.Cfg.Widgets.Clock {
		right = s.Secondary.Render(m.clock())
	}
	gap := m.width - ui.Width(left) - ui.Width(right)
	if gap < 1 {
		return ui.Truncate(left, m.width)
	}
	return left + strings.Repeat(" ", gap) + right
}

func (m *Model) footer() string {
	s := m.ctx.S
	var keys []ui.Key
	if a := m.active(); a != nil {
		keys = a.Keys()
		if !a.Capturing() {
			keys = append(keys, ui.Key{Keys: "?", Desc: "help"})
		}
	} else {
		keys = []ui.Key{{Keys: "↑↓", Desc: "move"}, {Keys: "enter", Desc: "open"}, {Keys: "0-9", Desc: "jump"},
			{Keys: "ctrl+p", Desc: "palette"}, {Keys: "?", Desc: "help"}, {Keys: "q", Desc: "quit"}}
	}
	left := s.HelpLine(keys)
	right := ""
	if m.notice.Text != "" {
		right = s.LevelStyle(m.notice.Level).Render(ui.Truncate(m.notice.Text, max(10, m.width/2)))
	}
	if m.quitAsk {
		right = s.Warning.Render(ui.T("Quit TermVerse? (y/n)"))
	}
	gap := m.width - ui.Width(left) - ui.Width(right)
	if gap < 1 {
		if right != "" {
			return ui.Truncate(right, m.width)
		}
		return ui.Truncate(left, m.width)
	}
	return left + strings.Repeat(" ", gap) + right
}

// View renders the whole screen.
func (m *Model) View() string {
	if m.width == 0 {
		return ""
	}
	s := m.ctx.S
	if m.width < 40 || m.height < 12 {
		msg := ui.Tf("Terminal too small (%dx%d). Minimum 40x12.", m.width, m.height)
		return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, s.Warning.Render(msg))
	}
	bodyH := m.height - 3
	var body string
	if a := m.active(); a != nil {
		body = ui.Fit(a.View(m.width, bodyH), m.width, bodyH)
	} else {
		body, m.layout = dashboard.Render(m.ctx, &m.menu, m.width, bodyH, m.now)
		body = ui.Fit(body, m.width, bodyH)
	}
	if m.showHelp {
		body = overlay(body, m.helpView(), m.width, bodyH)
	}
	if m.pal != nil {
		body = overlay(body, m.pal.view(s, m.width, bodyH), m.width, bodyH)
	}
	return m.header() + "\n" + s.Sep(m.width) + "\n" + body + "\n" + m.footer()
}

func (m *Model) helpView() string {
	s := m.ctx.S
	global := []ui.Key{
		{Keys: "↑/↓" + map[bool]string{true: " j/k", false: ""}[m.ctx.Vim()], Desc: "Move selection"},
		{Keys: "Enter", Desc: "Open / confirm"},
		{Keys: "0 … 9", Desc: "Open section directly (dashboard)"},
		{Keys: "Esc", Desc: "Back to previous screen"},
		{Keys: "Ctrl+P", Desc: "Command palette"},
		{Keys: "?", Desc: "Show / hide this help"},
		{Keys: "q", Desc: "Quit (from the dashboard)"},
		{Keys: "Ctrl+C", Desc: "Quit immediately"},
	}
	var sb strings.Builder
	sb.WriteString(s.Title.Render(ui.T("Global")) + "\n")
	for _, k := range global {
		sb.WriteString(s.Key.Render(ui.PadRight(k.Keys, 14)) + s.Text.Render(ui.T(k.Desc)) + "\n")
	}
	if a := m.active(); a != nil {
		sb.WriteString("\n" + s.Title.Render(a.Title()) + "\n")
		for _, k := range a.Keys() {
			sb.WriteString(s.Key.Render(ui.PadRight(k.Keys, 14)) + s.Text.Render(ui.T(k.Desc)) + "\n")
		}
	}
	sb.WriteString("\n" + s.Muted.Render(ui.T("Press any key to close")))
	content := strings.TrimRight(sb.String(), "\n")
	h := min(lipgloss.Height(content)+2, m.height-4)
	return s.Panel(ui.T("Keyboard shortcuts"), content, min(64, m.width-4), h, true)
}

// overlay draws box centered over base.
func overlay(base, box string, width, height int) string {
	bl := strings.Split(base, "\n")
	ol := strings.Split(box, "\n")
	bw := lipgloss.Width(box)
	top := max(0, (height-len(ol))/2)
	left := max(0, (width-bw)/2)
	for i, l := range ol {
		y := top + i
		if y >= len(bl) {
			break
		}
		prefix := ansi.Truncate(bl[y], left, "")
		prefix = ui.PadRight(prefix, left)
		bl[y] = prefix + l + strings.Repeat(" ", max(0, width-left-lipgloss.Width(l)))
	}
	return strings.Join(bl, "\n")
}

func textinputBlink() tea.Cmd { return nil }
