package ui

import (
	"github.com/charmbracelet/lipgloss"

	"github.com/KAYTAY10ok/TermVerse/internal/config"
	"github.com/KAYTAY10ok/TermVerse/internal/theme"
)

// Styles are the lipgloss styles derived from the active theme.
type Styles struct {
	C theme.Colors

	Primary, Secondary, Accent, Text, Muted lipgloss.Style
	Success, Warning, Error                 lipgloss.Style
	Title, Bold, Key, Selected, Disabled    lipgloss.Style

	Borders     bool
	Border      lipgloss.Border
	BorderColor lipgloss.Color
	Separator   string
	Selector    string
}

// BorderFor maps a style name to a lipgloss border.
func BorderFor(name string) lipgloss.Border {
	switch name {
	case "normal":
		return lipgloss.NormalBorder()
	case "double":
		return lipgloss.DoubleBorder()
	case "thick":
		return lipgloss.ThickBorder()
	default:
		return lipgloss.RoundedBorder()
	}
}

// NewStyles builds styles from a theme and appearance preferences.
func NewStyles(t theme.Theme, a config.Appearance) Styles {
	c := t.Colors
	fg := func(hex string) lipgloss.Style { return lipgloss.NewStyle().Foreground(lipgloss.Color(hex)) }
	return Styles{
		C:           c,
		Primary:     fg(c.Primary),
		Secondary:   fg(c.Secondary),
		Accent:      fg(c.Accent),
		Text:        fg(c.Text),
		Muted:       fg(c.Muted),
		Success:     fg(c.Success),
		Warning:     fg(c.Warning),
		Error:       fg(c.Error),
		Title:       fg(c.Primary).Bold(true),
		Bold:        fg(c.Text).Bold(true),
		Key:         fg(c.Accent).Bold(true),
		Selected:    lipgloss.NewStyle().Foreground(lipgloss.Color(c.HighlightText)).Background(lipgloss.Color(c.Highlight)).Bold(true),
		Disabled:    fg(c.Muted).Faint(true),
		Borders:     a.Borders,
		Border:      BorderFor(a.BorderStyle),
		BorderColor: lipgloss.Color(c.Border),
		Separator:   a.Separator,
		Selector:    a.Selector,
	}
}

// LevelStyle returns the style for a notification level.
func (s Styles) LevelStyle(l Level) lipgloss.Style {
	switch l {
	case Success:
		return s.Success
	case Warn:
		return s.Warning
	case Error:
		return s.Error
	default:
		return s.Secondary
	}
}

// Percent returns a style colored by threshold (green/yellow/red).
func (s Styles) Percent(p float64) lipgloss.Style {
	switch {
	case p >= 85:
		return s.Error
	case p >= 60:
		return s.Warning
	default:
		return s.Success
	}
}
