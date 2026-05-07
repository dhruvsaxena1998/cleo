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
}

func NewSpawnPopup(projectID string, agents []string) SpawnPopup {
	sorted := append([]string(nil), agents...)
	sort.Strings(sorted)
	ti := textinput.New()
	ti.Placeholder = "(optional)"
	ti.CharLimit = 64
	return SpawnPopup{agents: sorted, nameInput: ti, projectID: projectID}
}

type SpawnSubmitted struct {
	ProjectID string
	Agent     string
	Name      string
}
type SpawnCancelled struct{}

func (p SpawnPopup) Init() tea.Cmd { return textinput.Blink }

func (p SpawnPopup) View() string {
	var b strings.Builder
	fmt.Fprintf(&b, "Spawn agent in '%s'\n\n", p.projectID)
	b.WriteString("Agent:\n")
	for i, a := range p.agents {
		marker := "  "
		if i == p.cursor && !p.focusName {
			marker = "▸ "
		}
		b.WriteString(marker + a + "\n")
	}
	b.WriteString("\nName: ")
	b.WriteString(p.nameInput.View())
	b.WriteString("\n\ntab switch field   ↵ spawn   esc cancel")
	return lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).Padding(1, 2).Render(b.String())
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
			if !p.focusName && p.cursor > 0 {
				p.cursor--
			}
			return p, nil
		case "down", "j":
			if !p.focusName && p.cursor < len(p.agents)-1 {
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
