package customization

import "github.com/charmbracelet/lipgloss"

func lipColor(hex string) lipgloss.Color { return lipgloss.Color(hex) }

func joinH(a, b string) string { return lipgloss.JoinHorizontal(lipgloss.Top, a, " ", b) }
