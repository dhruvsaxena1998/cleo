package tui

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/dhruvsaxena1998/cleo/internal/config"
)

// WarningsPopup shows the boot-time config resolution outcomes: ✓ lines for
// what ended up active and ✗ lines for what the config asked for that did not
// take effect (unknown theme, dropped/conflicting/reserved keybinds, clamped
// values). It is opened by New() whenever the loaded config produced warnings.
type WarningsPopup struct {
	theme       Theme
	diagnostics []config.Diagnostic
}
type WarningsClosed struct{}

func NewWarningsPopup(theme Theme, diagnostics []config.Diagnostic) WarningsPopup {
	return WarningsPopup{theme: theme, diagnostics: diagnostics}
}

func (p WarningsPopup) Init() tea.Cmd { return nil }

func (p WarningsPopup) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if k, ok := msg.(tea.KeyMsg); ok {
		switch k.String() {
		case "esc", "q", "enter":
			return p, func() tea.Msg { return WarningsClosed{} }
		}
	}
	return p, nil
}

func (p WarningsPopup) View() string {
	const popW = 62
	cw := popW - 4

	okGlyph := lipgloss.NewStyle().Foreground(p.theme.Green).Bold(true).Render("✓")
	noGlyph := lipgloss.NewStyle().Foreground(p.theme.Red).Bold(true).Render("✗")
	headerStyle := lipgloss.NewStyle().Foreground(p.theme.Overlay0)
	detailStyle := lipgloss.NewStyle().Foreground(p.theme.Subtext0)
	// Budget for the detail text: row inner width minus the "  ✓  " prefix
	// (2 + 1 glyph + 2). Truncate the plain detail before styling so the frame's
	// width-based truncation never slices through an embedded ANSI sequence.
	detailW := cw - 5

	rows := []string{
		"",
		headerStyle.Render("What changed when Cleo loaded your config"),
		"",
	}
	for _, d := range p.diagnostics {
		glyph := noGlyph
		if d.OK {
			glyph = okGlyph
		}
		rows = append(rows, "  "+glyph+"  "+detailStyle.Render(truncateWidth(d.Detail, detailW)))
	}
	rows = append(rows, "")

	return drawFrame(frameSpec{
		Width:    popW,
		Title:    lipgloss.NewStyle().Foreground(p.theme.Accent).Bold(true).Render("Config notices"),
		Hint:     lipgloss.NewStyle().Foreground(p.theme.Overlay0).Render("esc / q to close"),
		Border:   popupBorderStyle(p.theme),
		Sections: [][]string{rows},
	})
}
