package tui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type ConfirmPopup struct {
	prompt string
	target string
}

func NewConfirmPopup(prompt, target string) ConfirmPopup {
	return ConfirmPopup{prompt: prompt, target: target}
}

type ConfirmYes struct{ Target string }
type ConfirmNo struct{}

func (p ConfirmPopup) Init() tea.Cmd { return nil }

func (p ConfirmPopup) View() string {
	var b strings.Builder
	b.WriteString(p.prompt + "\n\n")
	b.WriteString("y to confirm   esc/n to cancel")
	return lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).Padding(1, 2).Render(b.String())
}

func (p ConfirmPopup) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if k, ok := msg.(tea.KeyMsg); ok {
		switch k.String() {
		case "y", "Y":
			return p, func() tea.Msg { return ConfirmYes{Target: p.target} }
		case "esc", "n", "N":
			return p, func() tea.Msg { return ConfirmNo{} }
		}
	}
	return p, nil
}
