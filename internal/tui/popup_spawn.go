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

func NewSpawnPopup(projectID string, projectList []projects.Project, cwd string, agents []string, defaultAgent string, theme Theme) SpawnPopup {
	sorted := append([]string(nil), agents...)
	sort.Strings(sorted)

	pi := textinput.New()
	pi.Placeholder = "enter project path"
	pi.CharLimit = 256
	pi.Width = 56
	pi.ShowSuggestions = true
	pi.CompletionStyle = lipgloss.NewStyle().Foreground(theme.Overlay0)
	pi.KeyMap.AcceptSuggestion = key.NewBinding(key.WithKeys("right"))
	pi.KeyMap.NextSuggestion = key.NewBinding() // disable — we use l/right for agents
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

	// Resolve initial path and focus based on whether a project is selected.
	// Agent precedence: the project's own default wins, then the global
	// default_agent, then the first agent (agentIndex returns 0 when nothing
	// matches). The global default must apply in both branches — otherwise
	// opening the popup on a project with no per-project default would ignore
	// default_agent and silently fall back to the alphabetically-first agent.
	effectiveAgent := defaultAgent
	if projectID != "" {
		// Existing project: prefill path, start on label.
		for _, proj := range projectList {
			if proj.ID == projectID {
				p.pathInput.SetValue(proj.Path)
				if proj.DefaultAgent != "" {
					effectiveAgent = proj.DefaultAgent
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
	if effectiveAgent != "" {
		p.cursor = agentIndex(sorted, effectiveAgent)
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
	cw := popW - 4

	gold := lipgloss.NewStyle().Foreground(p.theme.Gold).Bold(true).Render("›")
	overlay := lipgloss.NewStyle().Foreground(p.theme.Overlay0)

	// ── 1. Path ────────────────────────────────────────────────────────────
	pathRows := []string{
		"",
		overlay.Render("1. path"),
		"   " + gold + " " + p.pathInput.View(),
	}
	if p.pathError != "" {
		pathRows = append(pathRows, "   "+lipgloss.NewStyle().Foreground(p.theme.Red).Bold(true).Render(p.pathError))
	}
	pathRows = append(pathRows, "")

	// ── 2. Label ────────────────────────────────────────────────────────────
	labelRows := []string{
		"",
		overlay.Render("2. label ") + overlay.Render("(optional)"),
		"   " + gold + " " + p.nameInput.View(),
		"",
	}

	// ── 3. AI Agent ─────────────────────────────────────────────────────────
	agentRows := []string{"", overlay.Render("3. ai-agent")}
	if p.focusIndex == 2 {
		// Focused: show as a horizontal selector like settings themes.
		agentRows = append(agentRows, "   "+lipgloss.NewStyle().Foreground(p.theme.Accent).Bold(true).Render("‹ "+p.agents[p.cursor]+" ›"))
	} else {
		// Unfocused: show the selected agent dimly.
		agentRows = append(agentRows, "   "+lipgloss.NewStyle().Foreground(p.theme.Subtext0).Render(p.agents[p.cursor]))
	}
	agentRows = append(agentRows, "")

	// ── Preview ─────────────────────────────────────────────────────────────
	previewRows := []string{""}
	if resolvedID := p.resolveProjectID(); resolvedID == "" {
		previewRows = append(previewRows, lipgloss.NewStyle().Foreground(p.theme.Mauve).Bold(true).Render("will register project, then create session"))
	} else if p.cursor < len(p.agents) {
		a := p.agents[p.cursor]
		name := strings.TrimSpace(p.nameInput.Value())
		if name == "" {
			name = "1"
		}
		sessID := fmt.Sprintf("cleo-%s-%s-%s", resolvedID, a, name)
		previewRows = append(previewRows,
			overlay.Render("will create  ")+lipgloss.NewStyle().Foreground(p.theme.Accent).Bold(true).Render(truncateWidth(sessID, cw-14)),
			overlay.Render(fmt.Sprintf("$ tmux new-session -d -s %s %s", sessID, a)),
		)
	}
	previewRows = append(previewRows, "")

	foot := p.theme.KeyHint("tab", "next field") + "  " + p.theme.KeyHint("←/→", "switch agent") + "  " +
		p.theme.KeyHint("enter", "spawn") + "  " + p.theme.KeyHint("esc", "cancel")

	return drawFrame(frameSpec{
		Width:    popW,
		Title:    lipgloss.NewStyle().Foreground(p.theme.Accent).Bold(true).Render("New Session"),
		Hint:     lipgloss.NewStyle().Foreground(p.theme.Overlay0).Render("spawn tmux-backed agent"),
		Border:   popupBorderStyle(p.theme),
		Sections: [][]string{pathRows, labelRows, agentRows, previewRows, {foot}},
	})
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
		case "left", "h":
			if p.focusIndex != 2 {
				break
			}
			if p.cursor > 0 {
				p.cursor--
			}
			return p, nil
		case "right", "l":
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
