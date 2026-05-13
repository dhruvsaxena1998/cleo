package tui

import (
	"fmt"
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type SpawnPopup struct {
	agents    []string
	cursor    int
	nameInput textinput.Model
	focusName bool
	projectID string
	theme     Theme
}

func NewSpawnPopup(projectID string, agents []string, theme Theme) SpawnPopup {
	sorted := append([]string(nil), agents...)
	sort.Strings(sorted)
	ti := textinput.New()
	ti.Placeholder = "optional — auto-generated if empty"
	ti.CharLimit = 64
	return SpawnPopup{agents: sorted, nameInput: ti, projectID: projectID, theme: theme}
}

type SpawnSubmitted struct {
	ProjectID string
	Agent     string
	Name      string
}
type SpawnCancelled struct{}

func (p SpawnPopup) Init() tea.Cmd { return textinput.Blink }

func (p SpawnPopup) View() string {
	const popW = 52
	bdr := lipgloss.NewStyle().Foreground(p.theme.Overlay1)
	iw := popW - 2
	cw := iw - 2

	var b strings.Builder

	hbar := strings.Repeat("─", iw)
	b.WriteString(bdr.Render("┌"+hbar+"┐") + "\n")
	titleLeft := lipgloss.NewStyle().Foreground(p.theme.Accent).Bold(true).Render("New Session")
	titleRight := lipgloss.NewStyle().Foreground(p.theme.Overlay0).Render("spawn tmux-backed agent")
	gap := cw - lipgloss.Width(titleLeft) - lipgloss.Width(titleRight)
	if gap < 0 {
		gap = 0
	}
	b.WriteString(bdr.Render("│") + " " + titleLeft + strings.Repeat(" ", gap) + titleRight + " " + bdr.Render("│") + "\n")
	b.WriteString(bdr.Render("├"+hbar+"┤") + "\n")

	blank := func() { b.WriteString(bdr.Render("│") + " " + strings.Repeat(" ", cw) + " " + bdr.Render("│") + "\n") }
	row := func(s string) {
		b.WriteString(bdr.Render("│") + " " + padRight(truncateWidth(s, cw), cw) + " " + bdr.Render("│") + "\n")
	}

	blank()
	row(lipgloss.NewStyle().Foreground(p.theme.Overlay0).Render("project  ") +
		lipgloss.NewStyle().Foreground(p.theme.Accent).Bold(true).Render(p.projectID))
	blank()
	b.WriteString(bdr.Render("├"+hbar+"┤") + "\n")

	blank()
	row(lipgloss.NewStyle().Foreground(p.theme.Overlay0).Render("1. label ") +
		lipgloss.NewStyle().Foreground(p.theme.Overlay0).Render("(optional)"))
	row("   " + lipgloss.NewStyle().Foreground(p.theme.Gold).Bold(true).Render("›") + " " + p.nameInput.View())
	blank()
	b.WriteString(bdr.Render("├"+hbar+"┤") + "\n")

	blank()
	row(lipgloss.NewStyle().Foreground(p.theme.Overlay0).Render("2. ai-agent"))
	selectedSt := lipgloss.NewStyle().Background(p.theme.Surf0).Foreground(p.theme.Text).Bold(true)
	dimSt := lipgloss.NewStyle().Foreground(p.theme.Subtext0)
	for i, a := range p.agents {
		active := i == p.cursor && !p.focusName
		var line string
		if active {
			line = selectedSt.Width(cw).Render(fmt.Sprintf("  ● %s", a))
		} else {
			line = dimSt.Render(fmt.Sprintf("  ○ %s", a))
		}
		row(line)
	}
	blank()
	b.WriteString(bdr.Render("├"+hbar+"┤") + "\n")

	blank()
	sessID := "—"
	agentCmd := "—"
	if p.cursor < len(p.agents) {
		a := p.agents[p.cursor]
		name := strings.TrimSpace(p.nameInput.Value())
		if name == "" {
			name = "1"
		}
		sessID = fmt.Sprintf("cleo-%s-%s-%s", p.projectID, a, name)
		agentCmd = a
	}
	row(lipgloss.NewStyle().Foreground(p.theme.Overlay0).Render("will create  ") +
		lipgloss.NewStyle().Foreground(p.theme.Accent).Bold(true).Render(truncateWidth(sessID, cw-14)))
	row(lipgloss.NewStyle().Foreground(p.theme.Overlay0).Render("$ tmux new-session -d -s"))
	row(lipgloss.NewStyle().Foreground(p.theme.Overlay0).Render(fmt.Sprintf("    %s %s", sessID, agentCmd)))
	blank()
	b.WriteString(bdr.Render("├"+hbar+"┤") + "\n")

	foot := p.theme.KeyHint("tab", "switch") + "  " + p.theme.KeyHint("j/k", "move") + "  " +
		p.theme.KeyHint("enter", "spawn") + "  " + p.theme.KeyHint("esc", "cancel")
	row(foot)
	b.WriteString(bdr.Render("└" + hbar + "┘"))

	return b.String()
}

func (p SpawnPopup) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			return p, func() tea.Msg { return SpawnCancelled{} }
		case "tab":
			p.focusName = !p.focusName
			if p.focusName {
				p.nameInput.Focus()
			} else {
				p.nameInput.Blur()
			}
			return p, nil
		case "enter":
			if len(p.agents) == 0 {
				return p, func() tea.Msg { return SpawnCancelled{} }
			}
			return p, func() tea.Msg {
				return SpawnSubmitted{
					ProjectID: p.projectID,
					Agent:     p.agents[p.cursor],
					Name:      strings.TrimSpace(p.nameInput.Value()),
				}
			}
		case "up", "k":
			if p.focusName {
				break
			}
			if p.cursor > 0 {
				p.cursor--
			}
			return p, nil
		case "down", "j":
			if p.focusName {
				break
			}
			if p.cursor < len(p.agents)-1 {
				p.cursor++
			}
			return p, nil
		}
	}
	if p.focusName {
		var cmd tea.Cmd
		p.nameInput, cmd = p.nameInput.Update(msg)
		return p, cmd
	}
	return p, nil
}
