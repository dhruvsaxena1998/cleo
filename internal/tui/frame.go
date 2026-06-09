package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// frameSpec describes a bordered box to draw. It is the single source of all
// box-drawing in the TUI: PanelBox and every popup are thin adapters that fill
// in a spec and hand it to drawFrame, instead of hand-rolling their own border
// glyphs, gap math, and row padding.
//
// Title and Hint are PRE-STYLED strings — the caller owns their colour, the
// frame only lays them out (left title, right-aligned hint). Sections holds the
// body as groups of pre-styled rows; a ├──┤ divider is drawn between adjacent
// groups, so a body+footer popup is two sections and a single-list popup is
// one. Border and Fill carry the look: popups pass a Mauve bold border and an
// empty fill (bare spaces); panels pass a Surf1 border and a Base-background
// fill. The core never branches on "popup vs panel" — that distinction lives
// entirely in the styles the caller passes.
type frameSpec struct {
	Width         int            // total outer width in cells
	Title         string         // left side of the title row (pre-styled)
	Hint          string         // right-aligned side of the title row (pre-styled)
	Sections      [][]string     // body rows, grouped; a divider separates adjacent groups
	Border        lipgloss.Style // style for the border glyphs
	Fill          lipgloss.Style // style for the inner region (background); zero value = none
	Height        int            // 0 = size to content; >0 = pad/truncate body to this height
	TitleInBorder bool           // bake the title into the top border line (the send popup)
}

// drawFrame draws spec as a bordered box and returns it newline-joined with no
// trailing newline. It owns the top/bottom border, the gap-filled title row, the
// dividers between sections, and per-row truncation+padding to the inner width.
func drawFrame(spec frameSpec) string {
	iw := spec.Width - 2
	if iw < 4 {
		iw = 4
	}
	cw := iw - 2
	hbar := strings.Repeat("─", iw)

	// One body row: the left border glyph, the Fill-styled inner slot (a leading
	// pad space, the content fitted to the content width, a trailing pad space),
	// then the right border glyph. An empty Fill renders the inner slot as bare
	// spaces (the popup look); a Base-background Fill paints it (the panel look).
	contentRow := func(content string) string {
		cell := padRight(truncateWidth(content, cw), cw)
		return spec.Border.Render("│") + spec.Fill.Render(" "+cell+" ") + spec.Border.Render("│")
	}
	divider := spec.Border.Render("├" + hbar + "┤")

	sections := spec.Sections
	// A fixed height applies to the single-section panel shape: pad with blank
	// rows or truncate so the whole box is exactly Height lines. Chrome is the
	// top border, the title row, its divider, and the bottom border — 4 lines.
	if spec.Height > 0 && !spec.TitleInBorder && len(sections) == 1 {
		contentH := spec.Height - 4
		if contentH < 0 {
			contentH = 0
		}
		body := make([]string, contentH)
		for i := 0; i < len(sections[0]) && i < contentH; i++ {
			body[i] = sections[0][i]
		}
		sections = [][]string{body}
	}

	var b strings.Builder
	if spec.TitleInBorder {
		// ┌ title ─────── hint ┐ — the caps are bare spaces flanking the (already
		// styled) title and hint; only the fill dashes carry the border style.
		leftCap := " " + spec.Title + " "
		rightCap := " " + spec.Hint + " "
		fill := iw - lipgloss.Width(leftCap) - lipgloss.Width(rightCap)
		if fill < 0 {
			fill = 0
		}
		b.WriteString(spec.Border.Render("┌") + leftCap + spec.Border.Render(strings.Repeat("─", fill)) + rightCap + spec.Border.Render("┐") + "\n")
	} else {
		b.WriteString(spec.Border.Render("┌"+hbar+"┐") + "\n")
		gap := cw - lipgloss.Width(spec.Title) - lipgloss.Width(spec.Hint)
		if gap < 0 {
			gap = 0
		}
		b.WriteString(contentRow(spec.Title+strings.Repeat(" ", gap)+spec.Hint) + "\n")
		b.WriteString(divider + "\n")
	}

	for i, sec := range sections {
		if i > 0 {
			b.WriteString(divider + "\n")
		}
		for _, row := range sec {
			b.WriteString(contentRow(row) + "\n")
		}
	}

	b.WriteString(spec.Border.Render("└" + hbar + "┘"))
	return b.String()
}
