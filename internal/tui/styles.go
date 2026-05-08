package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// ── Catppuccin Mocha ──────────────────────────────────────────────────────────

var (
	// Base surfaces
	clrBase    = lipgloss.Color("#1e1e2e")
	clrMantle  = lipgloss.Color("#181825")
	clrCrust   = lipgloss.Color("#11111b")
	clrSurf0   = lipgloss.Color("#313244")
	clrSurf1   = lipgloss.Color("#45475a")
	clrSurf2   = lipgloss.Color("#585b70")
	clrOverlay0 = lipgloss.Color("#6c7086")
	clrOverlay1 = lipgloss.Color("#7f849c")

	// Text
	clrText     = lipgloss.Color("#cdd6f4")
	clrSubtext1 = lipgloss.Color("#bac2de")
	clrSubtext0 = lipgloss.Color("#a6adc8")

	// Accent colours
	clrRosewater = lipgloss.Color("#f5e0dc")
	clrFlamingo  = lipgloss.Color("#f2cdcd")
	clrPink      = lipgloss.Color("#f5c2e7")
	clrMauve     = lipgloss.Color("#cba6f7")
	clrRed       = lipgloss.Color("#f38ba8")
	clrMaroon    = lipgloss.Color("#eba0ac")
	clrPeach     = lipgloss.Color("#fab387")
	clrYellow    = lipgloss.Color("#f9e2af")
	clrGreen     = lipgloss.Color("#a6e3a1")
	clrTeal      = lipgloss.Color("#94e2d5")
	clrSky       = lipgloss.Color("#89dceb")
	clrSapphire  = lipgloss.Color("#74c7ec")
	clrBlue      = lipgloss.Color("#89b4fa")
	clrLavender  = lipgloss.Color("#b4befe")
)

// ── Semantic aliases (used throughout the package) ────────────────────────────

var (
	clrPanel   = clrMantle
	clrRaised  = clrSurf0
	clrSubtext = clrSubtext0
	clrFaint   = clrOverlay0
	clrBorder  = clrSurf1

	clrSelBg = clrSurf0
	clrSelFg = clrText

	clrAccent = clrBlue    // titles, IDs, metrics
	clrGold   = clrYellow  // key hints

	// State colours
	clrAmber  = clrPeach   // waiting_for_input
	clrOrange = clrPeach   // spawning
	clrDimSt  = clrOverlay0 // dead
)

// ── Base styles ───────────────────────────────────────────────────────────────

var (
	styleDimmed = lipgloss.NewStyle().Foreground(clrSubtext0)
	styleFaint  = lipgloss.NewStyle().Foreground(clrOverlay0)
	styleBright = lipgloss.NewStyle().Foreground(clrText).Bold(true)

	styleApp     = lipgloss.NewStyle().Foreground(clrMauve).Bold(true)
	styleProject = lipgloss.NewStyle().Foreground(clrText).Bold(true)

	styleSelected = lipgloss.NewStyle().Background(clrSurf0).Foreground(clrText).Bold(true)

	// stylePanel kept for popup fallback
	stylePanel = lipgloss.NewStyle().
			Border(lipgloss.NormalBorder()).
			BorderForeground(clrSurf1).
			Padding(0, 1)

	styleTopbar = lipgloss.NewStyle().Background(clrMantle).Foreground(clrText).Padding(0, 1)

	styleKey    = lipgloss.NewStyle().Foreground(clrYellow).Bold(true)
	styleID     = lipgloss.NewStyle().Foreground(clrBlue).Bold(true)
	styleMetric = lipgloss.NewStyle().Foreground(clrBlue).Bold(true)
	styleError  = lipgloss.NewStyle().Foreground(clrRed)
)

// ── State helpers ─────────────────────────────────────────────────────────────

func stateColor(s string) lipgloss.Color {
	switch s {
	case "running":
		return clrGreen
	case "waiting_for_input":
		return clrPeach
	case "idle":
		return clrBlue
	case "spawning":
		return clrYellow
	case "completed":
		return clrGreen
	case "error":
		return clrRed
	case "dead":
		return clrOverlay0
	}
	return clrSubtext0
}

func stateGlyph(s string) string {
	switch s {
	case "running":
		return "●"
	case "waiting_for_input":
		return "◑"
	case "idle":
		return "○"
	case "spawning":
		return "◌"
	case "completed":
		return "✓"
	case "error":
		return "✗"
	case "dead":
		return "·"
	}
	return "·"
}

func styledGlyph(s string) string {
	return lipgloss.NewStyle().Foreground(stateColor(s)).Render(stateGlyph(s))
}

func styledStateText(s string) string {
	return lipgloss.NewStyle().Foreground(stateColor(s)).Render(s)
}

func agentLabel(label, color string) string {
	return lipgloss.NewStyle().Foreground(lipgloss.Color(color)).Bold(true).Render(label)
}

// agentBadge renders a solid coloured badge (Catppuccin accent bg + crust fg).
func agentBadge(label, bgColor string) string {
	return lipgloss.NewStyle().
		Background(lipgloss.Color(bgColor)).
		Foreground(clrCrust).
		Bold(true).
		Padding(0, 1).
		Render(label)
}

// ── Key hints & atoms ─────────────────────────────────────────────────────────

func keyHint(k, desc string) string {
	return styleKey.Render(k) + styleDimmed.Render(" "+desc)
}

func pill(label string, fg lipgloss.Color) string {
	return lipgloss.NewStyle().Foreground(fg).Background(clrMantle).Padding(0, 1).Render(label)
}

// ── String utilities ──────────────────────────────────────────────────────────

func padRight(s string, width int) string {
	if width <= 0 {
		return ""
	}
	w := lipgloss.Width(s)
	if w >= width {
		return s
	}
	return s + strings.Repeat(" ", width-w)
}

func truncateWidth(s string, width int) string {
	if width <= 0 {
		return ""
	}
	if lipgloss.Width(s) <= width {
		return s
	}
	if width <= 1 {
		return "…"
	}
	var b strings.Builder
	for _, r := range s {
		next := b.String() + string(r)
		if lipgloss.Width(next)+1 > width {
			break
		}
		b.WriteRune(r)
	}
	return b.String() + "…"
}

// ── Panel box ─────────────────────────────────────────────────────────────────
//
//	┌────────────────────────────────┐  ← border
//	│ Title                    hint  │  ← title row (Catppuccin Blue bold)
//	├────────────────────────────────┤  ← separator
//	│ content…                       │  ← h-4 content rows
//	└────────────────────────────────┘  ← border
func panelBox(title, hint string, body []string, w, h int) string {
	bdr := lipgloss.NewStyle().Foreground(clrSurf1)
	titleSt := lipgloss.NewStyle().Foreground(clrBlue).Bold(true)
	hintSt := styleFaint

	iw := w - 2
	if iw < 4 {
		iw = 4
	}

	// Title row (1 space padding each side → tUsable = iw-2)
	tUsable := iw - 2
	left := titleSt.Render(title)
	right := ""
	if hint != "" {
		right = hintSt.Render(hint)
	}
	gap := tUsable - lipgloss.Width(left) - lipgloss.Width(right)
	if gap < 0 {
		gap = 0
	}
	titleRow := left + strings.Repeat(" ", gap) + right

	// Content: clip / pad to h-4 rows
	contentH := h - 4
	if contentH < 0 {
		contentH = 0
	}
	lines := make([]string, contentH)
	for i := 0; i < len(body) && i < contentH; i++ {
		lines[i] = body[i]
	}

	hbar := strings.Repeat("─", iw)
	cUsable := iw - 2 // 1 space pad each side

	var b strings.Builder
	b.WriteString(bdr.Render("┌"+hbar+"┐") + "\n")
	b.WriteString(bdr.Render("│") + " " + padRight(titleRow, tUsable) + " " + bdr.Render("│") + "\n")
	b.WriteString(bdr.Render("├"+hbar+"┤") + "\n")
	for _, line := range lines {
		padded := padRight(line, cUsable)
		if lipgloss.Width(padded) > cUsable {
			padded = truncateWidth(padded, cUsable)
		}
		b.WriteString(bdr.Render("│") + " " + padded + " " + bdr.Render("│") + "\n")
	}
	b.WriteString(bdr.Render("└" + hbar + "┘"))
	return b.String()
}

func sectionDivider(label string, width int) string {
	head := styleFaint.Render("── " + label + " ")
	rem := width - lipgloss.Width(head)
	if rem < 1 {
		rem = 1
	}
	return head + styleFaint.Render(strings.Repeat("─", rem))
}
