package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/dhruvsaxena1998/cleo/internal/projects"
)

type SpawnPopup struct {
	pathInput  textinput.Model
	nameInput  textinput.Model
	agents     []string
	cursor     int // agent cursor
	focusIndex int // 0=path, 1=label, 2=agents
	projectID  string
	pathError  string
	projects   []projects.Project
	cwd        string
	theme      Theme
}

func NewSpawnPopup(projectID string, projectList []projects.Project, cwd string, agents []string, theme Theme) SpawnPopup {
	sorted := append([]string(nil), agents...)
	sort.Strings(sorted)

	pi := textinput.New()
	pi.Placeholder = "enter project path"
	pi.CharLimit = 256
	pi.Width = 56
	pi.ShowSuggestions = true
	pi.CompletionStyle = lipgloss.NewStyle().Foreground(theme.Overlay0)
	pi.KeyMap.AcceptSuggestion = key.NewBinding(key.WithKeys("right"))
	pi.KeyMap.NextSuggestion = key.NewBinding() // disable — we use j/k for agents
	pi.KeyMap.PrevSuggestion = key.NewBinding() // disable

	ni := textinput.New()
	ni.Placeholder = "optional — auto-generated if empty"
	ni.CharLimit = 32

	p := SpawnPopup{
		agents:    sorted,
		pathInput: pi,
		nameInput: ni,
		projectID: projectID,
		projects:  projectList,
		cwd:       cwd,
		theme:     theme,
	}

	// Resolve initial path and agent defaults based on project.
	if projectID != "" {
		// Existing project: prefill path, start on label, select project's default agent.
		for _, proj := range projectList {
			if proj.ID == projectID {
				p.pathInput.SetValue(proj.Path)
				if proj.DefaultAgent != "" {
					p.cursor = agentIndex(sorted, proj.DefaultAgent)
				}
				break
			}
		}
		p.focusIndex = 1 // label
		p.nameInput.Focus()
	} else {
		// No project: use CWD, start on path.
		if cwd != "" {
			p.pathInput.SetValue(cwd)
		}
		p.focusIndex = 0 // path
		p.pathInput.Focus()
	}

	p.updatePathSuggestions()

	return p
}

// agentIndex returns the index of agent in the sorted list, or 0 if not found.
func agentIndex(sorted []string, agent string) int {
	for i, a := range sorted {
		if a == agent {
			return i
		}
	}
	return 0
}

type SpawnSubmitted struct {
	ProjectID string // resolved from popup; empty string if path is new
	Path      string // raw path from input
	Agent     string
	Name      string
}

type SpawnCancelled struct{}

func (p SpawnPopup) Init() tea.Cmd { return textinput.Blink }

func (p SpawnPopup) View() string {
	const popW = 64
	bdr := popupBorderStyle(p.theme)
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

	blank := func() {
		b.WriteString(bdr.Render("│") + " " + strings.Repeat(" ", cw) + " " + bdr.Render("│") + "\n")
	}
	row := func(s string) {
		b.WriteString(bdr.Render("│") + " " + padRight(truncateWidth(s, cw), cw) + " " + bdr.Render("│") + "\n")
	}

	// ── 1. Path ────────────────────────────────────────────────────────────
	blank()
	pathLabel := lipgloss.NewStyle().Foreground(p.theme.Overlay0).Render("1. path")
	row(pathLabel)
	row("   " + lipgloss.NewStyle().Foreground(p.theme.Gold).Bold(true).Render("›") + " " + p.pathInput.View())
	if p.pathError != "" {
		errSt := lipgloss.NewStyle().Foreground(p.theme.Red).Bold(true)
		row("   " + errSt.Render(p.pathError))
	}
	blank()
	b.WriteString(bdr.Render("├"+hbar+"┤") + "\n")

	// ── 2. Label ────────────────────────────────────────────────────────────
	blank()
	row(lipgloss.NewStyle().Foreground(p.theme.Overlay0).Render("2. label ") +
		lipgloss.NewStyle().Foreground(p.theme.Overlay0).Render("(optional)"))
	row("   " + lipgloss.NewStyle().Foreground(p.theme.Gold).Bold(true).Render("›") + " " + p.nameInput.View())
	blank()
	b.WriteString(bdr.Render("├"+hbar+"┤") + "\n")

	// ── 3. AI Agent ─────────────────────────────────────────────────────────
	blank()
	row(lipgloss.NewStyle().Foreground(p.theme.Overlay0).Render("3. ai-agent"))
	selectedSt := lipgloss.NewStyle().Background(p.theme.Surf0).Foreground(p.theme.Text).Bold(true)
	dimSt := lipgloss.NewStyle().Foreground(p.theme.Subtext0)
	for i, a := range p.agents {
		active := i == p.cursor && p.focusIndex == 2
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

	// ── Preview ─────────────────────────────────────────────────────────────
	blank()
	resolvedID := p.resolveProjectID()
	if resolvedID == "" {
		row(lipgloss.NewStyle().Foreground(p.theme.Mauve).Bold(true).Render("will register project, then create session"))
	} else {
		if p.cursor < len(p.agents) {
			a := p.agents[p.cursor]
			name := strings.TrimSpace(p.nameInput.Value())
			if name == "" {
				name = "1"
			}
			sessID := fmt.Sprintf("cleo-%s-%s-%s", resolvedID, a, name)
			row(lipgloss.NewStyle().Foreground(p.theme.Overlay0).Render("will create  ") +
				lipgloss.NewStyle().Foreground(p.theme.Accent).Bold(true).Render(truncateWidth(sessID, cw-14)))
			row(lipgloss.NewStyle().Foreground(p.theme.Overlay0).Render(fmt.Sprintf("$ tmux new-session -d -s %s %s", sessID, a)))
		}
	}
	blank()
	b.WriteString(bdr.Render("├"+hbar+"┤") + "\n")

	foot := p.theme.KeyHint("tab", "next field") + "  " + p.theme.KeyHint("→", "complete path") + "  " + p.theme.KeyHint("j/k", "move agents") + "  " +
		p.theme.KeyHint("enter", "spawn") + "  " + p.theme.KeyHint("esc", "cancel")
	row(foot)
	b.WriteString(bdr.Render("└" + hbar + "┘"))

	return b.String()
}

// pathSuggestions returns a list of directory completions for the current path input.
// It scans the parent directory for child directories that share a prefix with the
// current input segment and returns full path suggestions.
func pathSuggestions(input string) []string {
	if input == "" {
		return nil
	}

	var dir, prefix string
	if strings.HasSuffix(input, "/") {
		dir = input
		prefix = ""
	} else {
		dir = filepath.Dir(input)
		prefix = filepath.Base(input)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}

	var suggestions []string
	for _, e := range entries {
		if e.IsDir() && strings.HasPrefix(e.Name(), prefix) {
			suggestions = append(suggestions, input+e.Name()[len(prefix):]+"/")
		}
	}
	return suggestions
}

// updatePathSuggestions recomputes filesystem suggestions for the path input.
func (p *SpawnPopup) updatePathSuggestions() {
	suggestions := pathSuggestions(p.pathInput.Value())
	if len(suggestions) > 0 {
		p.pathInput.SetSuggestions(suggestions)
	} else {
		p.pathInput.SetSuggestions(nil)
	}
}

// resolveProjectID checks if the current path matches an existing project.
// Returns the project ID if found, or empty string if the path is new/unregistered.
func (p SpawnPopup) resolveProjectID() string {
	path := strings.TrimSpace(p.pathInput.Value())
	for _, proj := range p.projects {
		if proj.Path == path {
			return proj.ID
		}
	}
	return ""
}

func (p SpawnPopup) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			return p, func() tea.Msg { return SpawnCancelled{} }
		case "tab":
			p.focusIndex = (p.focusIndex + 1) % 3
			p.updateFocus()
			return p, nil
		case "shift+tab":
			p.focusIndex = (p.focusIndex + 2) % 3 // +2 mod 3 = -1 mod 3
			p.updateFocus()
			return p, nil
		case "enter":
			if len(p.agents) == 0 {
				return p, func() tea.Msg { return SpawnCancelled{} }
			}
			// Validate path
			path := strings.TrimSpace(p.pathInput.Value())
			if path == "" {
				p.pathError = "path is required"
				return p, nil
			}
			info, err := os.Stat(path)
			if err != nil {
				p.pathError = "directory not found"
				return p, nil
			}
			if !info.IsDir() {
				p.pathError = "not a directory"
				return p, nil
			}
			// Clear any previous error
			p.pathError = ""
			resolvedID := p.resolveProjectID()
			return p, func() tea.Msg {
				return SpawnSubmitted{
					ProjectID: resolvedID,
					Path:      path,
					Agent:     p.agents[p.cursor],
					Name:      strings.TrimSpace(p.nameInput.Value()),
				}
			}
		case "up", "k":
			if p.focusIndex != 2 { // only move agents when agents section is focused
				break
			}
			if p.cursor > 0 {
				p.cursor--
			}
			return p, nil
		case "down", "j":
			if p.focusIndex != 2 {
				break
			}
			if p.cursor < len(p.agents)-1 {
				p.cursor++
			}
			return p, nil
		}
	default:
		// Non-key messages: forward to the focused input.
	}

	// Clear path error when the user types in the path field
	if p.focusIndex == 0 && p.pathError != "" {
		p.pathError = ""
	}

	// Forward input events to the focused text input
	if p.focusIndex == 0 {
		var cmd tea.Cmd
		p.pathInput, cmd = p.pathInput.Update(msg)
		p.updatePathSuggestions()
		return p, cmd
	}
	if p.focusIndex == 1 {
		var cmd tea.Cmd
		p.nameInput, cmd = p.nameInput.Update(msg)
		p.nameInput.SetValue(strings.ReplaceAll(p.nameInput.Value(), " ", "-"))
		return p, cmd
	}
	return p, nil
}

// updateFocus applies focus/blur to text inputs based on focusIndex.
func (p *SpawnPopup) updateFocus() {
	switch p.focusIndex {
	case 0:
		p.pathInput.Focus()
		p.nameInput.Blur()
		p.updatePathSuggestions()
	case 1:
		p.pathInput.Blur()
		p.nameInput.Focus()
		// Clear suggestions when leaving path field
		p.pathInput.SetSuggestions(nil)
	case 2:
		p.pathInput.Blur()
		p.nameInput.Blur()
		p.pathInput.SetSuggestions(nil)
	}
}
