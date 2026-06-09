package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/dhruvsaxena1998/cleo/internal/events"
)

// ── State helpers ─────────────────────────────────────────────────────────────

// stateGlyph returns the glyph for a session state from the active icon set, so
// the marker restyles with ui.icons rather than being hard-coded.
func (t Theme) stateGlyph(s string) string {
	switch s {
	case "running":
		return t.Icons.Running
	case "waiting_for_input":
		return t.Icons.Waiting
	case "idle":
		return t.Icons.Idle
	case "spawning":
		return t.Icons.Spawning
	case "completed":
		return t.Icons.Completed
	case "error":
		return t.Icons.Error
	case "dead":
		return t.Icons.Dead
	}
	return t.Icons.Dead
}

func agentLabel(label, color string) string {
	return lipgloss.NewStyle().Foreground(lipgloss.Color(color)).Bold(true).Render(label)
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
	return lipgloss.NewStyle().Foreground(t.StateColor(s)).Render(t.stateGlyph(s))
}

// animGlyph returns the marker for a state, animating the "working" states
// (running, spawning) through the icon set's spinner frames at the current
// animFrame. Every other state returns its static glyph. It is a Model method
// because the frame counter lives on the model, advanced by the spinner tick.
func (m Model) animGlyph(s string) string {
	if s == "running" || s == "spawning" {
		frames := m.theme.Icons.spinner()
		return frames[m.animFrame%len(frames)]
	}
	return m.theme.stateGlyph(s)
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

// KeyHint renders a footer key→label pair. The key gets a Surf0 background so it
// reads as a small key-cap chip, while the visible text stays exactly
// "<key> <desc>" with no inserted padding — footer assertions match that raw
// text, and the chip is purely an SGR background.
func (t Theme) KeyHint(k, desc string) string {
	key := lipgloss.NewStyle().Foreground(t.Gold).Background(t.Surf0).Bold(true).Render(k)
	sub := lipgloss.NewStyle().Foreground(t.Subtext0).Render(" " + desc)
	return key + sub
}

func popupBorderStyle(t Theme) lipgloss.Style {
	return lipgloss.NewStyle().Foreground(t.Mauve).Bold(true)
}

// PanelBox draws a dashboard panel — a bordered box with a title row and a
// fixed-height body. It is a thin adapter over the shared frame core: the panel
// look (Surf1 border, Base-filled inner) is expressed as the Border/Fill styles,
// and the panel's fixed height is the frame's Height. Its exported interface is
// unchanged, so the panel call sites stay untouched.
func (t Theme) PanelBox(title, hint string, body []string, w, h int) string {
	titleSt := lipgloss.NewStyle().Foreground(t.Accent).Bold(true)
	hintStr := ""
	if hint != "" {
		hintStr = lipgloss.NewStyle().Foreground(t.Overlay0).Render(hint)
	}
	return drawFrame(frameSpec{
		Width:    w,
		Title:    titleSt.Render(title),
		Hint:     hintStr,
		Sections: [][]string{body},
		// The Base background on both the border glyphs and the inner fill makes
		// transparent cells show the theme colour rather than the terminal default.
		Border: lipgloss.NewStyle().Foreground(t.Surf1).Background(t.Base),
		Fill:   lipgloss.NewStyle().Background(t.Base),
		Height: h,
	})
}

func (t Theme) SectionDivider(label string, width int) string {
	st := lipgloss.NewStyle().Foreground(t.Overlay0).Background(t.Base)
	head := st.Render("── " + label + " ")
	rem := width - lipgloss.Width(head)
	if rem < 1 {
		rem = 1
	}
	return head + st.Render(strings.Repeat("─", rem))
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
