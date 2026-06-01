package tui

import (
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

const sendPopupWidth = 84

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
	ti.Prompt = ""
	ti.Placeholder = "type message"
	ti.CharLimit = 4096
	ti.Width = sendPopupInputViewportWidth(theme)
	ti.Focus()

	return SendPopup{
		sessionID: sessionID,
		input:     ti,
		theme:     theme,
	}
}

func (p SendPopup) Init() tea.Cmd { return textinput.Blink }

func (p SendPopup) View() string {
	bdr := popupBorderStyle(p.theme)
	iw := sendPopupWidth - 2
	cw := iw - 2

	var b strings.Builder

	title := lipgloss.NewStyle().Foreground(p.theme.Text).Bold(true).Render("Quick Message")
	sid := lipgloss.NewStyle().Foreground(p.theme.Overlay0).Render(truncateWidth(p.sessionID, cw-lipgloss.Width(title)-3))
	topLabel := " " + title + " "
	topRight := " " + sid + " "
	fill := iw - lipgloss.Width(topLabel) - lipgloss.Width(topRight)
	if fill < 0 {
		fill = 0
	}
	b.WriteString(bdr.Render("┌") + topLabel + bdr.Render(strings.Repeat("─", fill)) + topRight + bdr.Render("┐") + "\n")

	blank := func() {
		b.WriteString(bdr.Render("│") + " " + strings.Repeat(" ", cw) + " " + bdr.Render("│") + "\n")
	}
	blank()

	prompt := lipgloss.NewStyle().Foreground(p.theme.Gold).Bold(true).Render("›")
	inputSlot := sendPopupInputSlotWidth(p.theme)
	input := p.input
	input.Prompt = ""
	input.Width = sendPopupInputViewportWidth(p.theme)
	inputView := padRight(ansi.Truncate(input.View(), inputSlot, ""), inputSlot)
	line := prompt + " " + inputView
	b.WriteString(bdr.Render("│") + " " + padRight(ansi.Truncate(line, cw, ""), cw) + " " + bdr.Render("│") + "\n")

	blank()

	hint := sendPopupHint(p.theme)
	hintGap := cw - lipgloss.Width(hint)
	if hintGap < 0 {
		hintGap = 0
		hint = ansi.Truncate(hint, cw, "")
	}
	b.WriteString(bdr.Render("│") + " " + strings.Repeat(" ", hintGap) + hint + " " + bdr.Render("│") + "\n")

	b.WriteString(bdr.Render("└" + strings.Repeat("─", iw) + "┘"))

	return b.String()
}

func sendPopupHint(theme Theme) string {
	return theme.KeyHint("enter", "send") + "  " + theme.KeyHint("esc", "cancel")
}

func sendPopupInputSlotWidth(theme Theme) int {
	cw := sendPopupWidth - 4
	promptWidth := lipgloss.Width("› ")
	return cw - promptWidth
}

func sendPopupInputViewportWidth(theme Theme) int {
	// textinput renders the cursor in one extra cell, so its viewport must be
	// one cell narrower than the slot reserved in the popup row.
	return sendPopupInputSlotWidth(theme) - 1
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
	p.input.Prompt = ""
	p.input.Width = sendPopupInputViewportWidth(p.theme)
	var cmd tea.Cmd
	p.input, cmd = p.input.Update(msg)
	return p, cmd
}
