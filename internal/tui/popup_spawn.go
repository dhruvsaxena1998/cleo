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
	ti.Placeholder = "optional — auto-generated if empty"
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
	const popW = 52
	bdr := lipgloss.NewStyle().Foreground(clrSurf1)
	cyn := lipgloss.NewStyle().Foreground(clrBlue).Bold(true)
	iw := popW - 2
	cw := iw - 2 // 1 space pad each side

	var b strings.Builder

	// ── Title bar ─────────────────────────────────────────
	hbar := strings.Repeat("─", iw)
	b.WriteString(bdr.Render("┌"+hbar+"┐") + "\n")
	titleLeft := cyn.Render("New Session")
	titleRight := styleFaint.Render("spawn tmux-backed agent")
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

	// ── Project ───────────────────────────────────────────
	blank()
	row(styleFaint.Render("project  ") + styleID.Render(p.projectID))
	blank()
	b.WriteString(bdr.Render("├"+hbar+"┤") + "\n")

	// ── Label field ───────────────────────────────────────
	blank()
	row(styleFaint.Render("1. label ") + styleFaint.Render("(optional)"))
	row("   " + styleKey.Render("›") + " " + p.nameInput.View())
	blank()
	b.WriteString(bdr.Render("├"+hbar+"┤") + "\n")

	// ── Agent list ────────────────────────────────────────
	blank()
	row(styleFaint.Render("2. ai-agent"))
	for i, a := range p.agents {
		active := i == p.cursor && !p.focusName
		var line string
		if active {
			line = styleSelected.Width(cw).Render(fmt.Sprintf("  ● %s", a))
		} else {
			line = styleDimmed.Render(fmt.Sprintf("  ○ %s", a))
		}
		row(line)
	}
	blank()
	b.WriteString(bdr.Render("├"+hbar+"┤") + "\n")

	// ── Preview ───────────────────────────────────────────
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
	row(styleFaint.Render("will create  ") + styleID.Render(truncateWidth(sessID, cw-14)))
	row(styleFaint.Render(fmt.Sprintf("$ tmux new-session -d -s %s %s",
		truncateWidth(sessID, 22), agentCmd)))
	blank()
	b.WriteString(bdr.Render("├"+hbar+"┤") + "\n")

	// ── Footer ────────────────────────────────────────────
	foot := keyHint("tab", "switch") + "  " + keyHint("j/k", "move") + "  " + keyHint("enter", "spawn") + "  " + keyHint("esc", "cancel")
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
