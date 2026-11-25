package ui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

// Item is an entry of a List.
type Item struct {
	Title    string
	Desc     string
	Right    string // optional right-aligned text
	Disabled bool
	Value    any
}

// List is a scrollable, keyboard-driven selection list.
type List struct {
	Items  []Item
	Cursor int
	offset int
	Vim    bool
}

// Selected returns the current item (zero Item if empty).
func (l *List) Selected() (Item, bool) {
	if l.Cursor < 0 || l.Cursor >= len(l.Items) {
		return Item{}, false
	}
	return l.Items[l.Cursor], true
}

// SetItems replaces the items, keeping the cursor in range.
func (l *List) SetItems(items []Item) {
	l.Items = items
	l.Clamp()
}

// Clamp keeps the cursor inside the list.
func (l *List) Clamp() {
	if l.Cursor >= len(l.Items) {
		l.Cursor = len(l.Items) - 1
	}
	if l.Cursor < 0 {
		l.Cursor = 0
	}
}

// Move moves the cursor by delta, wrapping around.
func (l *List) Move(delta int) {
	n := len(l.Items)
	if n == 0 {
		return
	}
	l.Cursor = ((l.Cursor+delta)%n + n) % n
}

// IsUp/IsDown report navigation keys, honoring vim keys.
func IsUp(k string, vim bool) bool   { return k == "up" || (vim && k == "k") }
func IsDown(k string, vim bool) bool { return k == "down" || (vim && k == "j") }

// HandleKey processes navigation keys and reports whether the key was used.
func (l *List) HandleKey(msg tea.KeyMsg) bool {
	k := msg.String()
	switch {
	case IsUp(k, l.Vim):
		l.Move(-1)
	case IsDown(k, l.Vim):
		l.Move(1)
	case k == "home" || k == "g" && l.Vim:
		l.Cursor = 0
	case k == "end" || k == "G" && l.Vim:
		l.Cursor = len(l.Items) - 1
	case k == "pgup":
		l.Cursor = max(0, l.Cursor-10)
	case k == "pgdown":
		l.Cursor = min(len(l.Items)-1, l.Cursor+10)
	default:
		return false
	}
	l.Clamp()
	return true
}

// HandleMouse scrolls with the wheel.
func (l *List) HandleMouse(msg tea.MouseMsg) bool {
	switch msg.Button {
	case tea.MouseButtonWheelUp:
		l.Move(-1)
	case tea.MouseButtonWheelDown:
		l.Move(1)
	default:
		return false
	}
	l.Clamp()
	return true
}

// View renders the list inside width×height.
func (l *List) View(s Styles, width, height int) string {
	if len(l.Items) == 0 {
		return s.Muted.Render(T("(empty)"))
	}
	twoLine := false
	for _, it := range l.Items {
		if it.Desc != "" {
			twoLine = true
			break
		}
	}
	per := 1
	if twoLine {
		per = 2
	}
	visible := max(1, height/per)
	if l.Cursor < l.offset {
		l.offset = l.Cursor
	}
	if l.Cursor >= l.offset+visible {
		l.offset = l.Cursor - visible + 1
	}
	if l.offset > len(l.Items)-visible {
		l.offset = max(0, len(l.Items)-visible)
	}
	var out []string
	for i := l.offset; i < len(l.Items) && i < l.offset+visible; i++ {
		it := l.Items[i]
		sel := i == l.Cursor
		prefix := "  "
		if sel {
			prefix = s.Selector + " "
		}
		left := prefix + it.Title
		right := it.Right
		gap := width - Width(left) - Width(right) - 1
		line := left
		if right != "" && gap > 0 {
			line = left + strings.Repeat(" ", gap) + right + " "
		}
		line = PadRight(Truncate(line, width), width)
		switch {
		case sel:
			line = s.Selected.Render(line)
		case it.Disabled:
			line = s.Disabled.Render(line)
		default:
			line = s.Text.Render(line)
		}
		out = append(out, line)
		if twoLine {
			out = append(out, "    "+s.Muted.Render(Truncate(it.Desc, width-4)))
		}
	}
	if len(l.Items) > visible {
		out = append(out, s.Muted.Render(Tf("%d/%d", l.Cursor+1, len(l.Items))))
	}
	return strings.Join(out, "\n")
}
