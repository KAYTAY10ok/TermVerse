package ui

import tea "github.com/charmbracelet/bubbletea"

// Cmd is an alias of tea.Cmd for brevity.
type Cmd = tea.Cmd

// Batch is tea.Batch.
func Batch(cmds ...tea.Cmd) tea.Cmd { return tea.Batch(cmds...) }
