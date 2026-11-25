package ui

import (
	"fmt"
	"math"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/lucasb-eyer/go-colorful"
)

// Truncate cuts a (possibly styled) string to width cells.
func Truncate(s string, width int) string {
	if width <= 0 {
		return ""
	}
	if ansi.StringWidth(s) <= width {
		return s
	}
	return ansi.Truncate(s, width, "…")
}

// Fit truncates every line to width and keeps at most height lines,
// padding with empty lines so the block has exactly height lines.
func Fit(s string, width, height int) string {
	lines := strings.Split(s, "\n")
	if height >= 0 && len(lines) > height {
		lines = lines[:height]
	}
	for i, l := range lines {
		lines[i] = PadRight(Truncate(l, width), width)
	}
	for height >= 0 && len(lines) < height {
		lines = append(lines, strings.Repeat(" ", max(width, 0)))
	}
	return strings.Join(lines, "\n")
}

// PadRight pads s with spaces to width cells.
func PadRight(s string, width int) string {
	w := ansi.StringWidth(s)
	if w >= width {
		return s
	}
	return s + strings.Repeat(" ", width-w)
}

// Center centers a single line in width.
func Center(s string, width int) string {
	w := ansi.StringWidth(s)
	if w >= width {
		return Truncate(s, width)
	}
	left := (width - w) / 2
	return strings.Repeat(" ", left) + s
}

// Panel draws a titled box of exactly width×height cells.
func (s Styles) Panel(title, body string, width, height int, focused bool) string {
	if width < 4 || height < 2 {
		return Fit(body, max(width, 0), max(height, 0))
	}
	titleStyle := s.Title
	if !focused {
		titleStyle = s.Secondary.Bold(true)
	}
	if !s.Borders {
		head := titleStyle.Render(Truncate(title, width))
		sep := s.Muted.Render(strings.Repeat(s.Separator, width))
		inner := Fit(body, width, height-2)
		return head + "\n" + sep + "\n" + inner
	}
	b := s.Border
	bc := lipgloss.NewStyle().Foreground(s.BorderColor)
	if focused {
		bc = lipgloss.NewStyle().Foreground(lipgloss.Color(s.C.Primary))
	}
	innerW := width - 2
	var top string
	if title != "" {
		t := " " + Truncate(title, innerW-4) + " "
		fill := innerW - 1 - ansi.StringWidth(t)
		if fill < 0 {
			fill = 0
		}
		top = bc.Render(b.TopLeft+b.Top) + titleStyle.Render(t) + bc.Render(strings.Repeat(b.Top, fill)+b.TopRight)
	} else {
		top = bc.Render(b.TopLeft + strings.Repeat(b.Top, innerW) + b.TopRight)
	}
	inner := Fit(body, innerW-2, height-2)
	var sb strings.Builder
	sb.WriteString(top)
	for _, l := range strings.Split(inner, "\n") {
		sb.WriteString("\n" + bc.Render(b.Left) + " " + l + " " + bc.Render(b.Right))
	}
	sb.WriteString("\n" + bc.Render(b.BottomLeft+strings.Repeat(b.Bottom, innerW)+b.BottomRight))
	return sb.String()
}

// PanelInner returns the usable content size of a Panel of width×height.
func (s Styles) PanelInner(width, height int) (int, int) {
	if !s.Borders {
		return width, height - 2
	}
	return width - 4, height - 2
}

// Bar renders a progress bar of the given width with a percentage label.
func (s Styles) Bar(pct float64, width int) string {
	if math.IsNaN(pct) {
		pct = 0
	}
	pct = math.Max(0, math.Min(100, pct))
	label := fmt.Sprintf(" %5.1f%%", pct)
	bw := width - len(label)
	if bw < 3 {
		return s.Percent(pct).Render(strings.TrimSpace(label))
	}
	filled := int(math.Round(pct / 100 * float64(bw)))
	return s.Percent(pct).Render(strings.Repeat("█", filled)) +
		s.Muted.Render(strings.Repeat("░", bw-filled)) + s.Text.Render(label)
}

// KV renders aligned "key  value" rows.
func (s Styles) KV(rows [][2]string, keyWidth int) string {
	var sb strings.Builder
	for i, r := range rows {
		if i > 0 {
			sb.WriteByte('\n')
		}
		sb.WriteString(s.Muted.Render(PadRight(r[0], keyWidth)) + " " + s.Text.Render(r[1]))
	}
	return sb.String()
}

// Sep renders a horizontal separator.
func (s Styles) Sep(width int) string {
	return s.Muted.Render(strings.Repeat(s.Separator, max(width, 0)))
}

// HelpLine renders "key desc · key desc" hints.
func (s Styles) HelpLine(keys []Key) string {
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, s.Key.Render(k.Keys)+" "+s.Muted.Render(T(k.Desc)))
	}
	return strings.Join(parts, s.Muted.Render("  ·  "))
}

// Tabs renders a tab bar.
func (s Styles) Tabs(names []string, active int) string {
	var parts []string
	for i, n := range names {
		label := " " + T(n) + " "
		if i == active {
			parts = append(parts, s.Selected.Render(label))
		} else {
			parts = append(parts, s.Muted.Render(label))
		}
	}
	return strings.Join(parts, s.Muted.Render("│"))
}

// Gradient colors each column of a multi-line block from color a to b.
// phase (0..1) shifts the gradient for a subtle animation.
func Gradient(block, a, b string, phase float64) string {
	ca, err1 := colorful.Hex(a)
	cb, err2 := colorful.Hex(b)
	if err1 != nil || err2 != nil {
		return block
	}
	lines := strings.Split(block, "\n")
	width := 0
	for _, l := range lines {
		width = max(width, len([]rune(l)))
	}
	if width == 0 {
		return block
	}
	cache := map[int]lipgloss.Style{}
	var sb strings.Builder
	for li, l := range lines {
		if li > 0 {
			sb.WriteByte('\n')
		}
		for x, r := range []rune(l) {
			if r == ' ' {
				sb.WriteRune(r)
				continue
			}
			t := float64(x)/float64(width) + phase
			t = t - math.Floor(t)
			t = 1 - math.Abs(2*t-1) // triangle wave: a → b → a
			key := int(t * 32)
			st, ok := cache[key]
			if !ok {
				st = lipgloss.NewStyle().Foreground(lipgloss.Color(ca.BlendLab(cb, float64(key)/32).Clamped().Hex()))
				cache[key] = st
			}
			sb.WriteString(st.Render(string(r)))
		}
	}
	return sb.String()
}
