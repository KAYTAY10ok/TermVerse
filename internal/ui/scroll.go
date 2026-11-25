package ui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

// Scroller is a simple vertically scrollable text view.
type Scroller struct {
	lines  []string
	Offset int
	height int
	Vim    bool
}

// SetContent replaces the content (keeps the offset in range).
func (sc *Scroller) SetContent(s string) {
	sc.lines = strings.Split(s, "\n")
	sc.clamp()
}

// Top scrolls to the beginning.
func (sc *Scroller) Top() { sc.Offset = 0 }

// Bottom scrolls to the end.
func (sc *Scroller) Bottom() { sc.Offset = len(sc.lines); sc.clamp() }

func (sc *Scroller) clamp() {
	maxOff := len(sc.lines) - max(1, sc.height)
	if sc.Offset > maxOff {
		sc.Offset = maxOff
	}
	if sc.Offset < 0 {
		sc.Offset = 0
	}
}

// HandleKey scrolls with arrows, page keys, home/end.
func (sc *Scroller) HandleKey(msg tea.KeyMsg) bool {
	k := msg.String()
	switch {
	case IsUp(k, sc.Vim):
		sc.Offset--
	case IsDown(k, sc.Vim):
		sc.Offset++
	case k == "pgup" || k == "b" && sc.Vim:
		sc.Offset -= max(1, sc.height-1)
	case k == "pgdown" || k == " " || k == "f" && sc.Vim:
		sc.Offset += max(1, sc.height-1)
	case k == "home" || k == "g" && sc.Vim:
		sc.Offset = 0
	case k == "end" || k == "G" && sc.Vim:
		sc.Offset = len(sc.lines)
	default:
		return false
	}
	sc.clamp()
	return true
}

// HandleMouse scrolls with the wheel.
func (sc *Scroller) HandleMouse(msg tea.MouseMsg) {
	switch msg.Button {
	case tea.MouseButtonWheelUp:
		sc.Offset -= 3
	case tea.MouseButtonWheelDown:
		sc.Offset += 3
	}
	sc.clamp()
}

// View renders the visible window with a scroll indicator.
func (sc *Scroller) View(s Styles, width, height int) string {
	sc.height = height
	sc.clamp()
	end := min(len(sc.lines), sc.Offset+height)
	vis := sc.lines[sc.Offset:end]
	out := Fit(strings.Join(vis, "\n"), width, height)
	if len(sc.lines) > height && height > 0 {
		pct := 100
		if d := len(sc.lines) - height; d > 0 {
			pct = sc.Offset * 100 / d
		}
		ls := strings.Split(out, "\n")
		ind := s.Muted.Render(Tf(" %d%% ", pct))
		last := ls[len(ls)-1]
		ls[len(ls)-1] = Truncate(last, width-Width(ind)) + ind
		out = strings.Join(ls, "\n")
	}
	return out
}

// Markdown renders a light subset of Markdown (headings, bullets, code).
func (s Styles) Markdown(md string, width int) string {
	var out []string
	inCode := false
	for _, l := range strings.Split(md, "\n") {
		t := strings.TrimSpace(l)
		switch {
		case strings.HasPrefix(t, "```"):
			inCode = !inCode
			continue
		case inCode:
			out = append(out, s.Accent.Render("  "+l))
		case strings.HasPrefix(t, "# "):
			out = append(out, s.Title.Render(strings.ToUpper(t[2:])))
		case strings.HasPrefix(t, "## "):
			out = append(out, "", s.Title.Render(t[3:]))
		case strings.HasPrefix(t, "### "):
			out = append(out, s.Secondary.Bold(true).Render(t[4:]))
		case strings.HasPrefix(t, "- ") || strings.HasPrefix(t, "* "):
			indent := len(l) - len(strings.TrimLeft(l, " "))
			for i, wl := range strings.Split(Wrap(t[2:], max(10, width-4-indent)), "\n") {
				bullet := "• "
				if i > 0 {
					bullet = "  "
				}
				out = append(out, strings.Repeat(" ", indent)+s.Accent.Render(bullet)+s.Text.Render(wl))
			}
		default:
			for _, wl := range strings.Split(Wrap(l, max(10, width)), "\n") {
				out = append(out, s.Text.Render(wl))
			}
		}
	}
	return strings.Join(out, "\n")
}
