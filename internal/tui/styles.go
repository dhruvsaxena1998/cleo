package tui

import "github.com/charmbracelet/lipgloss"

var (
	styleProject  = lipgloss.NewStyle().Bold(true)
	styleDimmed   = lipgloss.NewStyle().Faint(true)
	styleSelected = lipgloss.NewStyle().Reverse(true)
	stylePanel    = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).Padding(0, 1)
)

func agentLabel(label, color string) string {
	return lipgloss.NewStyle().Foreground(lipgloss.Color(color)).Render("[" + label + "]")
}

func stateGlyph(s string) string {
	switch s {
	case "running":
		return "●"
	case "waiting_for_input":
		return "◐"
	case "idle":
		return "○"
	case "completed":
		return "✓"
	case "error":
		return "✗"
	case "dead":
		return "☠"
	}
	return "·"
}
