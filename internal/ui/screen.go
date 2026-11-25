package ui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

// Key documents a keyboard shortcut for help screens and footers.
type Key struct {
	Keys string
	Desc string
}

// Screen is one full-screen section of TermVerse.
type Screen interface {
	// ID is the stable identifier (also used by the module registry).
	ID() string
	// Title is the translated display name.
	Title() string
	// Init is called every time the screen is opened.
	Init() tea.Cmd
	// Update handles a message. Screens return BackCmd to leave.
	Update(msg tea.Msg) (Screen, tea.Cmd)
	// View renders the body in the given area.
	View(width, height int) string
	// Keys lists the shortcuts of the current state.
	Keys() []Key
	// Capturing reports whether a text field has focus: global shortcuts
	// (?, q, digits, Ctrl+P) are then delivered to the screen instead.
	Capturing() bool
}

// Leaver is implemented by screens that must react when hidden (e.g. pause a game).
type Leaver interface{ OnLeave() }

// BackMsg asks the app to return to the previous screen.
type BackMsg struct{}

// BackCmd returns a BackMsg.
func BackCmd() tea.Msg { return BackMsg{} }

// NavigateMsg asks the app to open a screen by ID, optionally deep-linking
// to a sub-page ("devtools.uuid" → ID "devtools", Sub "uuid").
type NavigateMsg struct{ ID, Sub string }

// Navigate returns a command that opens the screen id ("section" or "section.sub").
func Navigate(id string) tea.Cmd {
	return func() tea.Msg {
		sec, sub, _ := strings.Cut(id, ".")
		return NavigateMsg{ID: sec, Sub: sub}
	}
}

// DeepLinker is implemented by screens that can open a sub-page directly.
type DeepLinker interface{ OpenSub(sub string) tea.Cmd }

// Level of a notification.
type Level int

// Notification levels.
const (
	Info Level = iota
	Success
	Warn
	Error
)

// NotifyMsg displays a transient message in the status bar.
type NotifyMsg struct {
	Text  string
	Level Level
}

// Notify returns a command producing a NotifyMsg.
func Notify(level Level, text string) tea.Cmd {
	return func() tea.Msg { return NotifyMsg{Text: text, Level: level} }
}

// ConfigChangedMsg is emitted after preferences change (theme, mouse, ...).
type ConfigChangedMsg struct{}

// ConfigChanged returns a ConfigChangedMsg command.
func ConfigChanged() tea.Msg { return ConfigChangedMsg{} }
