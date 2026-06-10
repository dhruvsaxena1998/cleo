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
	cw := sendPopupWidth - 4

	// The send popup is the one outlier: the title and session id are baked into
	// the top border line, and the body has no internal divider. The frame's
	// TitleInBorder variant reproduces that compact, airy shape.
	title := lipgloss.NewStyle().Foreground(p.theme.Text).Bold(true).Render("Quick Message")
	sid := lipgloss.NewStyle().Foreground(p.theme.Overlay0).Render(truncateWidth(p.sessionID, cw-lipgloss.Width(title)-3))

	prompt := lipgloss.NewStyle().Foreground(p.theme.Gold).Bold(true).Render("›")
	inputSlot := sendPopupInputSlotWidth(p.theme)
	input := p.input
	input.Prompt = ""
	input.Width = sendPopupInputViewportWidth(p.theme)
	inputView := padRight(ansi.Truncate(input.View(), inputSlot, ""), inputSlot)
	// Fit the input line to the content width with ANSI-aware truncation so the
	// (already styled) prompt and input never slice a colour escape mid-sequence.
	inputLine := padRight(ansi.Truncate(prompt+" "+inputView, cw, ""), cw)

	hint := sendPopupHint(p.theme)
	hintGap := cw - lipgloss.Width(hint)
	if hintGap < 0 {
		hintGap = 0
		hint = ansi.Truncate(hint, cw, "")
	}
	hintRow := strings.Repeat(" ", hintGap) + hint

	return drawFrame(frameSpec{
		Width:         sendPopupWidth,
		Title:         title,
		Hint:          sid,
		Border:        popupBorderStyle(p.theme),
		TitleInBorder: true,
		Sections:      [][]string{{"", inputLine, "", hintRow}},
	})
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

func (p SendPopup) withTheme(t Theme) tea.Model { p.theme = t; return p }

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
