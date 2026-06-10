package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/dhruvsaxena1998/cleo/internal/config"
)

type HelpPopup struct {
	theme     Theme
	keymap    config.Keymap
	detachKey string
}
type HelpClosed struct{}

func NewHelpPopup(theme Theme, km config.Keymap, detachKey string) HelpPopup {
	return HelpPopup{theme: theme, keymap: km, detachKey: formatTmuxKey(detachKey)}
}

// formatTmuxKey converts tmux notation (e.g. "C-b d") to a readable form ("ctrl+b d").
func formatTmuxKey(k string) string {
	return strings.NewReplacer("C-", "ctrl+", "M-", "alt+").Replace(k)
}

func (p HelpPopup) Init() tea.Cmd { return nil }

func (p HelpPopup) withTheme(t Theme) tea.Model { p.theme = t; return p }

func (p HelpPopup) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if k, ok := msg.(tea.KeyMsg); ok {
		switch k.String() {
		case "esc", "q", "?":
			return p, func() tea.Msg { return HelpClosed{} }
		}
	}
	return p, nil
}

func (p HelpPopup) View() string {
	const popW = 58

	type row struct{ key, desc string }
	km := p.keymap
	// Keys are derived from the resolved keymap (the single source of truth, so
	// rebinds show here); the descriptions stay tui-side presentation prose,
	// richer than the config registry's terse labels.
	act := func(b key.Binding, desc string) row { return row{keysLabel(b), desc} }
	sections := []struct {
		title string
		rows  []row
	}{
		{"Navigation", []row{
			act(km.Up, "up"),
			act(km.Down, "down"),
			act(km.Space, "expand / collapse"),
		}},
		{"Session Actions", []row{
			act(km.Enter, "attach"),
			act(km.Editor, "open Project in editor"),
			act(km.View, "view pane"),
			act(km.Send, "send message (single-line, attach for prompts)"),
			act(km.New, "new session"),
			act(km.Rename, "rename"),
			act(km.Kill, "kill session"),
			act(km.Prune, "prune finished"),
			act(km.Remove, "remove project"),
		}},
		{"Global", []row{
			act(km.Filter, "find"),
			act(km.Mute, "mute / unmute"),
			act(km.Settings, "settings"),
			act(km.Help, "help"),
			act(km.Quit, "quit"),
		}},
		{"tmux (inside a session)", []row{
			{p.detachKey, "detach — return to cleo"},
			{"ctrl+b [", "scroll mode  (q to exit)"},
			{"ctrl+b z", "zoom / unzoom pane"},
		}},
	}

	// Each keybinding section becomes one frame section (blank, section title,
	// the key rows, blank); the frame draws a divider between adjacent sections.
	frameSections := make([][]string, 0, len(sections))
	for _, sec := range sections {
		rows := []string{"", lipgloss.NewStyle().Foreground(p.theme.Overlay0).Render(sec.title)}
		for _, r := range sec.rows {
			rows = append(rows, fmt.Sprintf("  %s  %s",
				lipgloss.NewStyle().Foreground(p.theme.Gold).Bold(true).Render(r.key),
				lipgloss.NewStyle().Foreground(p.theme.Subtext0).Render(r.desc),
			))
		}
		rows = append(rows, "")
		frameSections = append(frameSections, rows)
	}

	return drawFrame(frameSpec{
		Width:    popW,
		Title:    lipgloss.NewStyle().Foreground(p.theme.Accent).Bold(true).Render("Keybindings"),
		Hint:     lipgloss.NewStyle().Foreground(p.theme.Overlay0).Render("esc / q to close"),
		Border:   popupBorderStyle(p.theme),
		Sections: frameSections,
	})
}
