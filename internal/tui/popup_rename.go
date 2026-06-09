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
	ti.CharLimit = 32
	ti.Focus()
	return RenamePopup{sessionID: sessionID, input: ti, theme: theme}
}

func (p RenamePopup) Init() tea.Cmd { return textinput.Blink }

func (p RenamePopup) View() string {
	const popW = 48
	cw := popW - 4

	title := lipgloss.NewStyle().Foreground(p.theme.Accent).Bold(true).Render("Rename Session")
	// The session id sits to the right of the title; truncate it to whatever
	// inner width the title leaves so the title row never overflows.
	sid := lipgloss.NewStyle().Foreground(p.theme.Overlay0).Render(truncateWidth(p.sessionID, cw-lipgloss.Width(title)-1))
	inputRow := "  " + lipgloss.NewStyle().Foreground(p.theme.Gold).Bold(true).Render("›") + " " + p.input.View()

	return drawFrame(frameSpec{
		Width:  popW,
		Title:  title,
		Hint:   sid,
		Border: popupBorderStyle(p.theme),
		Sections: [][]string{
			{"", lipgloss.NewStyle().Foreground(p.theme.Overlay0).Render("new name"), inputRow, ""},
			{p.theme.KeyHint("enter", "confirm") + "  " + p.theme.KeyHint("esc", "cancel")},
		},
	})
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
	p.input.SetValue(strings.ReplaceAll(p.input.Value(), " ", "-"))
	return p, cmd
}
