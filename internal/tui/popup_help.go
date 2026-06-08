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
	bdr := popupBorderStyle(p.theme)
	iw := popW - 2
	cw := iw - 2

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

	hbar := strings.Repeat("─", iw)
	var b strings.Builder

	b.WriteString(bdr.Render("┌"+hbar+"┐") + "\n")
	titleLeft := lipgloss.NewStyle().Foreground(p.theme.Accent).Bold(true).Render("Keybindings")
	titleRight := lipgloss.NewStyle().Foreground(p.theme.Overlay0).Render("esc / q to close")
	gap := cw - lipgloss.Width(titleLeft) - lipgloss.Width(titleRight)
	if gap < 0 {
		gap = 0
	}
	b.WriteString(bdr.Render("│") + " " + titleLeft + strings.Repeat(" ", gap) + titleRight + " " + bdr.Render("│") + "\n")
	b.WriteString(bdr.Render("├"+hbar+"┤") + "\n")

	writeRow := func(s string) {
		b.WriteString(bdr.Render("│") + " " + padRight(truncateWidth(s, cw), cw) + " " + bdr.Render("│") + "\n")
	}
	writeBlank := func() {
		b.WriteString(bdr.Render("│") + " " + strings.Repeat(" ", cw) + " " + bdr.Render("│") + "\n")
	}

	for si, sec := range sections {
		if si > 0 {
			b.WriteString(bdr.Render("├"+hbar+"┤") + "\n")
		}
		writeBlank()
		writeRow(lipgloss.NewStyle().Foreground(p.theme.Overlay0).Render(sec.title))
		for _, r := range sec.rows {
			line := fmt.Sprintf("  %s  %s",
				lipgloss.NewStyle().Foreground(p.theme.Gold).Bold(true).Render(r.key),
				lipgloss.NewStyle().Foreground(p.theme.Subtext0).Render(r.desc),
			)
			writeRow(line)
		}
		writeBlank()
	}

	b.WriteString(bdr.Render("└" + hbar + "┘"))
	return b.String()
}
