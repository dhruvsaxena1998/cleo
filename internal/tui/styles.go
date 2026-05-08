package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/dhruvsaxena1998/cleo/internal/events"
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
	clrPanel       = clrMantle
	clrRaised      = clrSurf0
	clrSubtext     = clrSubtext0
	clrFaint       = clrOverlay0
	clrBorder      = clrSurf1
	clrPopupBorder = clrOverlay1

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

// ── Theme rendering methods ───────────────────────────────────────────────────

func (t Theme) StateColor(s string) lipgloss.Color {
	switch s {
	case "running":
		return t.Green
	case "waiting_for_input":
		return t.Peach
	case "idle":
		return t.Blue
	case "spawning":
		return t.Yellow
	case "completed":
		return t.Green
	case "error":
		return t.Red
	case "dead":
		return t.Overlay0
	}
	return t.Subtext0
}

func (t Theme) StyledGlyph(s string) string {
	return lipgloss.NewStyle().Foreground(t.StateColor(s)).Render(stateGlyph(s))
}

func (t Theme) StyledStateText(s string) string {
	return lipgloss.NewStyle().Foreground(t.StateColor(s)).Render(s)
}

func (t Theme) AgentBadge(label, bgColor string) string {
	return lipgloss.NewStyle().
		Background(lipgloss.Color(bgColor)).
		Foreground(t.Crust).
		Bold(true).
		Padding(0, 1).
		Render(label)
}

func (t Theme) Pill(label string, fg lipgloss.Color) string {
	return lipgloss.NewStyle().Foreground(fg).Background(t.Mantle).Padding(0, 1).Render(label)
}

func (t Theme) KeyHint(k, desc string) string {
	key := lipgloss.NewStyle().Foreground(t.Gold).Bold(true).Render(k)
	sub := lipgloss.NewStyle().Foreground(t.Subtext0).Render(" " + desc)
	return key + sub
}

func (t Theme) PanelBox(title, hint string, body []string, w, h int) string {
	bdr := lipgloss.NewStyle().Foreground(t.Surf1)
	titleSt := lipgloss.NewStyle().Foreground(t.Accent).Bold(true)
	hintSt := lipgloss.NewStyle().Foreground(t.Overlay0)

	iw := w - 2
	if iw < 4 {
		iw = 4
	}

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

	contentH := h - 4
	if contentH < 0 {
		contentH = 0
	}
	lines := make([]string, contentH)
	for i := 0; i < len(body) && i < contentH; i++ {
		lines[i] = body[i]
	}

	hbar := strings.Repeat("─", iw)
	cUsable := iw - 2

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

func (t Theme) SectionDivider(label string, width int) string {
	faint := lipgloss.NewStyle().Foreground(t.Overlay0)
	head := faint.Render("── " + label + " ")
	rem := width - lipgloss.Width(head)
	if rem < 1 {
		rem = 1
	}
	return head + faint.Render(strings.Repeat("─", rem))
}

func (t Theme) EventTypeColor(evType string) lipgloss.Color {
	switch evType {
	case "PreToolUse", "pre_tool_use":
		return t.Peach
	case "PostToolUse", "post_tool_use":
		return t.Green
	case "Stop", "stop", "SessionEnd", "session_end", "idle_timeout":
		return t.Peach
	case "Notification", "notification", "user_resume":
		return t.Accent
	case "SessionStart", "session_start":
		return t.Accent
	case "error", "dead":
		return t.Red
	}
	return t.Subtext0
}

func (t Theme) FormatEventRow(e events.Entry, width int, highlight bool) string {
	ts := lipgloss.NewStyle().Foreground(t.Overlay0).Render(e.At.Format("15:04:05"))
	evType := lipgloss.NewStyle().Foreground(t.EventTypeColor(e.Type)).Render(fmt.Sprintf("%-16s", e.Type))
	detail := e.Detail
	if detail == "" {
		detail = e.Tool
	}
	dur := ""
	if e.DurationS > 0 {
		dur = lipgloss.NewStyle().Foreground(t.Overlay0).Render(fmt.Sprintf("%.1fs", e.DurationS))
	}

	durW := 6
	fixed := 9 + 2 + 16 + 2 + durW + 2
	detailW := width - fixed
	if detailW < 4 {
		detailW = 4
	}
	detailStr := lipgloss.NewStyle().Foreground(t.Subtext0).Render(truncateWidth(detail, detailW))
	if e.Type == "Notification" || e.Type == "notification" {
		detailStr = lipgloss.NewStyle().Foreground(t.Gold).Render(truncateWidth(detail, detailW))
	}

	row := ts + "  " + evType + "  " + padRight(detailStr, detailW) + "  " + padRight(dur, durW)
	if highlight {
		row = lipgloss.NewStyle().Background(t.Surf0).Foreground(t.Text).Bold(true).Render(row)
	}
	return row
}
