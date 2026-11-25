package ui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

// Option is a row of an Options form.
type Option struct {
	Label  string
	Header bool // section header (not selectable)
	Value  func() string
	Cycle  func(delta int) tea.Cmd // ←/→
	Enter  func() tea.Cmd          // Enter / space
	Hint   string
}

// Options is a vertical settings form.
type Options struct {
	Items  []Option
	Cursor int
	offset int
	Vim    bool
}

func (o *Options) selectable(i int) bool { return i >= 0 && i < len(o.Items) && !o.Items[i].Header }

// Fix moves the cursor to a selectable row.
func (o *Options) Fix() {
	if o.Cursor >= len(o.Items) {
		o.Cursor = len(o.Items) - 1
	}
	for i := o.Cursor; i < len(o.Items); i++ {
		if o.selectable(i) {
			o.Cursor = i
			return
		}
	}
	for i := o.Cursor; i >= 0; i-- {
		if o.selectable(i) {
			o.Cursor = i
			return
		}
	}
}

func (o *Options) move(d int) {
	n := len(o.Items)
	for step := 0; step < n; step++ {
		o.Cursor = ((o.Cursor+d)%n + n) % n
		if o.selectable(o.Cursor) {
			return
		}
	}
}

// Current returns the selected option.
func (o *Options) Current() *Option {
	if !o.selectable(o.Cursor) {
		return nil
	}
	return &o.Items[o.Cursor]
}

// HandleKey processes navigation and editing keys.
func (o *Options) HandleKey(msg tea.KeyMsg) (bool, tea.Cmd) {
	k := msg.String()
	cur := o.Current()
	switch {
	case IsUp(k, o.Vim):
		o.move(-1)
	case IsDown(k, o.Vim):
		o.move(1)
	case k == "left" || o.Vim && k == "h":
		if cur != nil && cur.Cycle != nil {
			return true, cur.Cycle(-1)
		}
	case k == "right" || o.Vim && k == "l":
		if cur != nil && cur.Cycle != nil {
			return true, cur.Cycle(1)
		}
	case k == "enter" || k == " ":
		if cur != nil {
			if cur.Enter != nil {
				return true, cur.Enter()
			}
			if cur.Cycle != nil {
				return true, cur.Cycle(1)
			}
		}
	default:
		return false, nil
	}
	return true, nil
}

// View renders the form.
func (o *Options) View(s Styles, width, height int) string {
	o.Fix()
	labelW := 0
	for _, it := range o.Items {
		if !it.Header {
			labelW = max(labelW, Width(T(it.Label)))
		}
	}
	labelW = min(labelW+2, width/2)
	if o.Cursor < o.offset {
		o.offset = o.Cursor
		if o.offset > 0 && o.Items[o.offset-1].Header {
			o.offset--
		}
	}
	if o.Cursor >= o.offset+height {
		o.offset = o.Cursor - height + 1
	}
	var out []string
	for i := o.offset; i < len(o.Items) && len(out) < height; i++ {
		it := o.Items[i]
		if it.Header {
			out = append(out, s.Title.Render(strings.ToUpper(T(it.Label))))
			continue
		}
		sel := i == o.Cursor
		prefix := "  "
		if sel {
			prefix = s.Selector + " "
		}
		label := PadRight(prefix+T(it.Label), labelW+2)
		val := ""
		if it.Value != nil {
			val = it.Value()
		}
		if it.Cycle != nil {
			val = "‹ " + val + " ›"
		} else if it.Enter != nil && val == "" {
			val = "↵"
		}
		if sel {
			out = append(out, s.Selected.Render(PadRight(Truncate(label+val, width), width)))
		} else {
			out = append(out, s.Text.Render(label)+s.Secondary.Render(Truncate(val, width-labelW-2)))
		}
	}
	return strings.Join(out, "\n")
}

// OnOff formats a boolean.
func OnOff(b bool) string {
	if b {
		return T("on")
	}
	return T("off")
}

// CycleIndex returns the index of v in list moved by delta (wrapping).
func CycleIndex[T comparable](list []T, v T, delta int) T {
	idx := 0
	for i, x := range list {
		if x == v {
			idx = i
			break
		}
	}
	n := len(list)
	return list[((idx+delta)%n+n)%n]
}
