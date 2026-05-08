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
	const popW = 44
	bdr := lipgloss.NewStyle().Foreground(clrBorder)
	iw := popW - 2

	var b strings.Builder
	b.WriteString(bdr.Render("┌"+strings.Repeat("─", iw)+"┐") + "\n")
	titleLeft := styleError.Render("Confirm Kill")
	titleRight := styleFaint.Render("destructive")
	gap := iw - lipgloss.Width(titleLeft) - lipgloss.Width(titleRight) - 2
	if gap < 1 {
		gap = 1
	}
	b.WriteString(bdr.Render("│") + " " + titleLeft + strings.Repeat(" ", gap) + titleRight + " " + bdr.Render("│") + "\n")
	b.WriteString(bdr.Render("├"+strings.Repeat("─", iw)+"┤") + "\n")

	cw := iw - 2
	b.WriteString(bdr.Render("│") + " " + strings.Repeat(" ", cw) + " " + bdr.Render("│") + "\n")
	prompt := truncateWidth(p.prompt, cw)
	b.WriteString(bdr.Render("│") + " " + padRight(styleDimmed.Render(prompt), cw) + " " + bdr.Render("│") + "\n")
	b.WriteString(bdr.Render("│") + " " + strings.Repeat(" ", cw) + " " + bdr.Render("│") + "\n")
	b.WriteString(bdr.Render("├"+strings.Repeat("─", iw)+"┤") + "\n")
	foot := keyHint("y", "confirm kill") + "   " + keyHint("esc", "cancel") + styleDimmed.Render(" / ") + keyHint("n", "cancel")
	b.WriteString(bdr.Render("│") + " " + padRight(truncateWidth(foot, cw), cw) + " " + bdr.Render("│") + "\n")
	b.WriteString(bdr.Render("└" + strings.Repeat("─", iw) + "┘"))
	return b.String()
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
