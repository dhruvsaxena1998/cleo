package tui

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type ConfirmPopup struct {
	title       string
	actionLabel string
	kind        string
	prompt      string
	target      string
	theme       Theme
}

func NewConfirmPopup(title, actionLabel, prompt, target, kind string, theme Theme) ConfirmPopup {
	return ConfirmPopup{title: title, actionLabel: actionLabel, prompt: prompt, target: target, kind: kind, theme: theme}
}

const (
	confirmKindKill          = "kill"
	confirmKindPrune         = "prune"
	confirmKindRemoveProject = "remove-project"
)

type ConfirmYes struct {
	Target string
	Kind   string
}
type ConfirmNo struct{}

func (p ConfirmPopup) Init() tea.Cmd { return nil }

func (p ConfirmPopup) View() string {
	const popW = 58
	cw := popW - 4

	titleLeft := lipgloss.NewStyle().Foreground(p.theme.Red).Render(p.title)
	titleRight := lipgloss.NewStyle().Foreground(p.theme.Overlay0).Render("destructive")
	prompt := lipgloss.NewStyle().Foreground(p.theme.Subtext0).Render(truncateWidth(p.prompt, cw))
	foot := p.theme.KeyHint("y", p.actionLabel) + "   " + p.theme.KeyHint("esc", "cancel") +
		lipgloss.NewStyle().Foreground(p.theme.Subtext0).Render(" / ") + p.theme.KeyHint("n", "cancel")

	return drawFrame(frameSpec{
		Width:  popW,
		Title:  titleLeft,
		Hint:   titleRight,
		Border: popupBorderStyle(p.theme),
		Sections: [][]string{
			{"", prompt, ""}, // body: a blank, the prompt, a blank
			{foot},           // footer hints, divided from the body
		},
	})
}

func (p ConfirmPopup) withTheme(t Theme) tea.Model { p.theme = t; return p }

func (p ConfirmPopup) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if k, ok := msg.(tea.KeyMsg); ok {
		switch k.String() {
		case "y", "Y":
			return p, func() tea.Msg { return ConfirmYes{Target: p.target, Kind: p.kind} }
		case "esc", "n", "N":
			return p, func() tea.Msg { return ConfirmNo{} }
		}
	}
	return p, nil
}
