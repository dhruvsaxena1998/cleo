package tui

import (
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

type SendPopup struct {
	sessionID string
	input     textinput.Model
	theme     Theme
}

type SendSubmitted struct {
	SessionID string
	Text      string
}

type SendCancelled struct{}

func NewSendPopup(sessionID string, theme Theme) SendPopup {
	ti := textinput.New()
	ti.Placeholder = "type your message…"
	ti.CharLimit = 4096
	ti.Width = 62
	ti.Focus()

	return SendPopup{
		sessionID: sessionID,
		input:     ti,
		theme:     theme,
	}
}

func (p SendPopup) Init() tea.Cmd { return textinput.Blink }

func (p SendPopup) View() string {
	const popW = 68
	bdr := lipgloss.NewStyle().Foreground(p.theme.Overlay1)
	iw := popW - 2
	cw := iw - 2

	hbar := strings.Repeat("─", iw)
	var b strings.Builder

	b.WriteString(bdr.Render("┌"+hbar+"┐") + "\n")
	titleLeft := lipgloss.NewStyle().Foreground(p.theme.Accent).Bold(true).Render("Send Message")
	titleRight := lipgloss.NewStyle().Foreground(p.theme.Overlay0).Render(truncateWidth(p.sessionID, cw-lipgloss.Width(titleLeft)-1))
	gap := cw - lipgloss.Width(titleLeft) - lipgloss.Width(titleRight)
	if gap < 0 {
		gap = 0
	}
	b.WriteString(bdr.Render("│") + " " + titleLeft + strings.Repeat(" ", gap) + titleRight + " " + bdr.Render("│") + "\n")
	b.WriteString(bdr.Render("├"+hbar+"┤") + "\n")

	// Blank line for spacing
	b.WriteString(bdr.Render("│") + " " + strings.Repeat(" ", cw) + " " + bdr.Render("│") + "\n")
	// Text input — pad to fixed width so borders stay aligned.
	// The textinput handles its own cursor scrolling internally.
	inputVis := "  " + p.input.View()
	b.WriteString(bdr.Render("│") + " " + padRight(ansi.Truncate(inputVis, cw, ""), cw) + " " + bdr.Render("│") + "\n")
	// Blank line for spacing
	b.WriteString(bdr.Render("│") + " " + strings.Repeat(" ", cw) + " " + bdr.Render("│") + "\n")

	b.WriteString(bdr.Render("├"+hbar+"┤") + "\n")
	foot := p.theme.KeyHint("enter", "send") + "  " + p.theme.KeyHint("esc", "cancel")
	b.WriteString(bdr.Render("│") + " " + padRight(truncateWidth(foot, cw), cw) + " " + bdr.Render("│") + "\n")
	b.WriteString(bdr.Render("└" + hbar + "┘"))

	return b.String()
}

func (p SendPopup) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyEsc:
			return p, func() tea.Msg { return SendCancelled{} }
		case tea.KeyEnter:
			text := strings.TrimSpace(p.input.Value())
			if text == "" {
				return p, nil
			}
			return p, func() tea.Msg {
				return SendSubmitted{SessionID: p.sessionID, Text: text}
			}
		}
	}
	var cmd tea.Cmd
	p.input, cmd = p.input.Update(msg)
	return p, cmd
}
