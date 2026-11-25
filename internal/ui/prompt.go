package ui

import (
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

// Prompt is a modal single-line input with a callback.
type Prompt struct {
	Active   bool
	Label    string
	Input    textinput.Model
	OnSubmit func(value string) tea.Cmd
}

// Open activates the prompt.
func (p *Prompt) Open(s Styles, label, placeholder, value string, onSubmit func(string) tea.Cmd) tea.Cmd {
	p.Active, p.Label, p.OnSubmit = true, label, onSubmit
	p.Input = s.NewInput(placeholder, 256)
	p.Input.SetValue(value)
	p.Input.CursorEnd()
	return p.Input.Focus()
}

// Update handles keys while active.
func (p *Prompt) Update(msg tea.Msg) tea.Cmd {
	if km, ok := msg.(tea.KeyMsg); ok {
		switch km.String() {
		case "esc":
			p.Active = false
			return nil
		case "enter":
			p.Active = false
			if p.OnSubmit != nil {
				return p.OnSubmit(p.Input.Value())
			}
			return nil
		}
	}
	var cmd tea.Cmd
	p.Input, cmd = p.Input.Update(msg)
	return cmd
}

// View renders the prompt.
func (p *Prompt) View(s Styles, width int) string {
	p.Input.Width = max(10, width-4)
	return s.Accent.Render(T(p.Label)) + "\n" + p.Input.View() + "\n" +
		s.Muted.Render(T("enter confirm · esc cancel"))
}

// Confirm is a yes/no question.
type Confirm struct {
	Active   bool
	Question string
	OnYes    func() tea.Cmd
}

// Ask activates the confirmation.
func (c *Confirm) Ask(q string, onYes func() tea.Cmd) { c.Active, c.Question, c.OnYes = true, q, onYes }

// Update handles y/n keys.
func (c *Confirm) Update(msg tea.Msg) tea.Cmd {
	km, ok := msg.(tea.KeyMsg)
	if !ok {
		return nil
	}
	switch km.String() {
	case "y", "Y", "o", "O":
		c.Active = false
		if c.OnYes != nil {
			return c.OnYes()
		}
	case "n", "N", "esc", "q":
		c.Active = false
	}
	return nil
}

// View renders the confirmation.
func (c *Confirm) View(s Styles) string {
	return s.Warning.Render("⚠ "+c.Question) + "\n" + s.Muted.Render(T("y = yes · n / esc = no"))
}
