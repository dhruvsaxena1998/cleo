package tui

import (
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type RenamePopup struct {
	sessionID string
	input     textinput.Model
	theme     Theme
}

type RenameSubmitted struct {
	SessionID string
	NewName   string
}
type RenameCancelled struct{}

func NewRenamePopup(sessionID, currentName string, theme Theme) RenamePopup {
	ti := textinput.New()
	ti.SetValue(currentName)
	ti.CharLimit = 64
	ti.Focus()
	return RenamePopup{sessionID: sessionID, input: ti, theme: theme}
}

func (p RenamePopup) Init() tea.Cmd { return textinput.Blink }

func (p RenamePopup) View() string {
	const popW = 48
	bdr := lipgloss.NewStyle().Foreground(p.theme.Overlay1)
	iw := popW - 2
	cw := iw - 2

	hbar := strings.Repeat("─", iw)
	var b strings.Builder

	b.WriteString(bdr.Render("┌"+hbar+"┐") + "\n")
	title := lipgloss.NewStyle().Foreground(p.theme.Accent).Bold(true).Render("Rename Session")
	sid := lipgloss.NewStyle().Foreground(p.theme.Overlay0).Render(truncateWidth(p.sessionID, cw-lipgloss.Width(title)-1))
	gap := cw - lipgloss.Width(title) - lipgloss.Width(sid)
	if gap < 0 {
		gap = 0
	}
	b.WriteString(bdr.Render("│") + " " + title + strings.Repeat(" ", gap) + sid + " " + bdr.Render("│") + "\n")
	b.WriteString(bdr.Render("├"+hbar+"┤") + "\n")

	row := func(s string) {
		b.WriteString(bdr.Render("│") + " " + padRight(truncateWidth(s, cw), cw) + " " + bdr.Render("│") + "\n")
	}
	blank := func() { row("") }

	blank()
	row(lipgloss.NewStyle().Foreground(p.theme.Overlay0).Render("new name"))
	row("  " + lipgloss.NewStyle().Foreground(p.theme.Gold).Bold(true).Render("›") + " " + p.input.View())
	blank()
	b.WriteString(bdr.Render("├"+hbar+"┤") + "\n")
	row(p.theme.KeyHint("enter", "confirm") + "  " + p.theme.KeyHint("esc", "cancel"))
	b.WriteString(bdr.Render("└" + hbar + "┘"))

	return b.String()
}

func (p RenamePopup) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			return p, func() tea.Msg { return RenameCancelled{} }
		case "enter":
			name := strings.TrimSpace(p.input.Value())
			if name == "" {
				return p, nil
			}
			return p, func() tea.Msg {
				return RenameSubmitted{SessionID: p.sessionID, NewName: name}
			}
		}
	}
	var cmd tea.Cmd
	p.input, cmd = p.input.Update(msg)
	return p, cmd
}
