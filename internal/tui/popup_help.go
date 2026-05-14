package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type HelpPopup struct {
	theme     Theme
	detachKey string
}
type HelpClosed struct{}

func NewHelpPopup(theme Theme, detachKey string) HelpPopup {
	return HelpPopup{theme: theme, detachKey: formatTmuxKey(detachKey)}
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
	const colW = 42
	bdr := lipgloss.NewStyle().Foreground(p.theme.Overlay1)

	hbar := strings.Repeat("─", colW+2)
	topBar := "┌" + hbar + "┬" + hbar + "┐"
	midBar := "├" + hbar + "┼" + hbar + "┤"
	botBar := "└" + hbar + "┴" + hbar + "┘"

	sectionSt := lipgloss.NewStyle().Foreground(p.theme.Overlay0)
	keySt := lipgloss.NewStyle().Foreground(p.theme.Gold).Bold(true)
	descSt := lipgloss.NewStyle().Foreground(p.theme.Subtext0)
	mauveSt := lipgloss.NewStyle().Foreground(p.theme.Mauve)

	// ── left column: inputs ──────────────────────────────────────────────────

	type krow struct{ key, desc string }
	type section struct {
		title string
		rows  []krow
	}
	leftSections := []section{
		{"Navigation", []krow{
			{"↑ / k", "up"},
			{"↓ / j", "down"},
			{"space", "expand / collapse"},
		}},
		{"Session Actions", []krow{
			{"↵", "attach"},
			{"v", "view pane"},
			{"n", "new session"},
			{"r", "rename"},
			{"K", "kill session"},
			{"P", "prune finished"},
			{"D", "remove project"},
		}},
		{"Global", []krow{
			{"/", "filter"},
			{"m", "mute / unmute"},
			{"?", "help"},
			{"q", "quit"},
		}},
		{"tmux", []krow{
			{p.detachKey, "detach — return to cleo"},
		}},
	}

	var left []string
	for si, sec := range leftSections {
		if si > 0 {
			left = append(left, "")
		}
		left = append(left, "")
		left = append(left, sectionSt.Render(sec.title))
		for _, r := range sec.rows {
			left = append(left, fmt.Sprintf("  %s  %s", keySt.Render(r.key), descSt.Render(r.desc)))
		}
		left = append(left, "")
	}

	// ── right column: reference ──────────────────────────────────────────────

	type irow struct {
		glyph string
		color lipgloss.Color
		desc  string
	}
	iconRows := []irow{
		{"◉", p.theme.Blue, "working"},
		{"⚠", p.theme.Gold, "needs input"},
		{"✓", p.theme.Green, "completed"},
		{"✗", p.theme.Red, "failed"},
		{"∙", p.theme.Overlay0, "idle"},
		{"○", p.theme.Overlay0, "stopped"},
	}

	var right []string
	right = append(right, "")
	right = append(right, sectionSt.Render("Icon Legend"))
	for _, ir := range iconRows {
		glyph := lipgloss.NewStyle().Foreground(ir.color).Bold(true).Render(ir.glyph)
		right = append(right, fmt.Sprintf("  %s  %s", glyph, descSt.Render(ir.desc)))
	}
	right = append(right, "")
	right = append(right, sectionSt.Render("Filter"))
	right = append(right, "  "+descSt.Render("type to match project · session · agent"))
	right = append(right, fmt.Sprintf("  %s%s%s",
		descSt.Render("case-insensitive · "),
		keySt.Render("esc"),
		descSt.Render(" to clear"),
	))
	right = append(right, "")
	right = append(right, sectionSt.Render("Config  ")+mauveSt.Render("~/.config/cleo/config.toml"))
	for _, key := range []string{
		"defaults.detach_key",
		"defaults.default_agent",
		"ui.theme",
		"ui.show_pane_preview",
		"agents.<name>",
	} {
		right = append(right, "  "+mauveSt.Render(key))
	}
	right = append(right, "")

	// ── pad shorter column ───────────────────────────────────────────────────

	for len(left) < len(right) {
		left = append(left, "")
	}
	for len(right) < len(left) {
		right = append(right, "")
	}

	// ── stitch ───────────────────────────────────────────────────────────────

	var b strings.Builder

	b.WriteString(bdr.Render(topBar) + "\n")

	// Title row: "Help" left-aligned in left cell, "esc / q to close" right-aligned in right cell.
	titleLeft := lipgloss.NewStyle().Foreground(p.theme.Accent).Bold(true).Render("Help")
	closeHint := lipgloss.NewStyle().Foreground(p.theme.Overlay0).Render("esc / q to close")
	gapClose := colW - lipgloss.Width(closeHint)
	if gapClose < 0 {
		gapClose = 0
	}
	b.WriteString(
		bdr.Render("│") + " " + padRight(titleLeft, colW) + " " +
			bdr.Render("│") + " " + strings.Repeat(" ", gapClose) + closeHint + " " +
			bdr.Render("│") + "\n",
	)
	b.WriteString(bdr.Render(midBar) + "\n")

	// Body rows.
	for i := range left {
		l := truncateWidth(left[i], colW)
		r := truncateWidth(right[i], colW)
		b.WriteString(
			bdr.Render("│") + " " + padRight(l, colW) + " " +
				bdr.Render("│") + " " + padRight(r, colW) + " " +
				bdr.Render("│") + "\n",
		)
	}

	b.WriteString(bdr.Render(botBar))
	return b.String()
}
