package ui

import (
	"os"
	"strings"
	"unicode"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/x/ansi"
)

// Width returns the display width of s.
func Width(s string) int { return ansi.StringWidth(s) }

// NewInput creates a themed single-line text input.
func (s Styles) NewInput(placeholder string, limit int) textinput.Model {
	ti := textinput.New()
	ti.Placeholder = T(placeholder)
	ti.CharLimit = limit
	ti.Prompt = s.Selector + " "
	ti.PromptStyle = s.Accent
	ti.TextStyle = s.Text
	ti.PlaceholderStyle = s.Muted
	return ti
}

// Sanitize removes control characters and escape sequences from untrusted
// text (files, network messages, command output) before display.
func Sanitize(s string) string {
	s = ansi.Strip(s)
	return strings.Map(func(r rune) rune {
		switch {
		case r == '\n' || r == '\t':
			return r
		case r == '\r':
			return -1
		case unicode.IsControl(r):
			return -1
		}
		return r
	}, s)
}

// Wrap hard-wraps plain text to width.
func Wrap(s string, width int) string {
	if width <= 0 {
		return s
	}
	return ansi.Hardwrap(ansi.Wordwrap(s, width, " "), width, true)
}

// Tildify replaces the home directory prefix of a path with "~" for display.
func Tildify(path string) string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" || home == "/" {
		return path
	}
	if path == home {
		return "~"
	}
	if strings.HasPrefix(path, home+string(os.PathSeparator)) {
		return "~" + path[len(home):]
	}
	return path
}
