package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/dhruvsaxena1998/cleo/internal/events"
	"github.com/dhruvsaxena1998/cleo/internal/state"
)

// ── Display state taxonomy ────────────────────────────────────────────────────

// DisplayState is the 6-value visual taxonomy shown in the TUI.
// Internal state.State enum values are NOT renamed — mapping happens here only.
type DisplayState int

const (
	DisplayNeedsInput DisplayState = iota // waiting_for_input → yellow ⚠
	DisplayWorking                        // running, spawning  → blue animated ✽
	DisplayIdle                           // idle               → dimmed ∙
	DisplayCompleted                      // completed          → green ✓
	DisplayFailed                         // error              → red ✗
	DisplayStopped                        // dead               → grey ○
)

// ToDisplayState maps internal state.State to one of the six display states.
func ToDisplayState(s state.State) DisplayState {
	switch s {
	case state.WaitingForInput:
		return DisplayNeedsInput
	case state.Running, state.Spawning:
		return DisplayWorking
	case state.Idle:
		return DisplayIdle
	case state.Completed:
		return DisplayCompleted
	case state.Errored:
		return DisplayFailed
	case state.Dead:
		return DisplayStopped
	}
	return DisplayStopped
}

// urgencyOrder returns a sort key: lower = higher urgency (sorts first).
func urgencyOrder(ds DisplayState) int { return int(ds) }

func displayStateGlyph(ds DisplayState) string {
	switch ds {
	case DisplayNeedsInput:
		return "⚠"
	case DisplayWorking:
		return "◉"
	case DisplayIdle:
		return "∙"
	case DisplayCompleted:
		return "✓"
	case DisplayFailed:
		return "✗"
	case DisplayStopped:
		return "○"
	}
	return "○"
}

// ── State helpers ─────────────────────────────────────────────────────────────

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

// DisplayStateColor returns the theme color for a DisplayState.
func (t Theme) DisplayStateColor(ds DisplayState) lipgloss.Color {
	switch ds {
	case DisplayNeedsInput:
		return t.Gold
	case DisplayWorking:
		return t.Blue
	case DisplayIdle:
		return t.Surf2
	case DisplayCompleted:
		return t.Green
	case DisplayFailed:
		return t.Red
	case DisplayStopped:
		return t.Overlay0
	}
	return t.Subtext0
}

func (t Theme) StyledGlyph(s string) string {
	ds := ToDisplayState(state.State(s))
	return lipgloss.NewStyle().Foreground(t.DisplayStateColor(ds)).Render(displayStateGlyph(ds))
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
	iw := w - 2
	if iw < 4 {
		iw = 4
	}
	cUsable := iw - 2
	tUsable := iw - 2

	// bdr carries the Base background so border characters fill with the theme colour
	// rather than the terminal default.
	bdr := lipgloss.NewStyle().Foreground(t.Surf1).Background(t.Base)
	titleSt := lipgloss.NewStyle().Foreground(t.Accent).Bold(true)
	hintSt := lipgloss.NewStyle().Foreground(t.Overlay0)
	// innerSt fills the full iw-wide slot between the two │ glyphs with Base bg;
	// Width(iw) ensures trailing spaces are explicitly styled, not transparent.
	innerSt := lipgloss.NewStyle().Background(t.Base).Width(iw)

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

	var b strings.Builder
	b.WriteString(bdr.Render("┌"+hbar+"┐") + "\n")
	b.WriteString(bdr.Render("│") + innerSt.Render(" "+padRight(titleRow, tUsable)) + bdr.Render("│") + "\n")
	b.WriteString(bdr.Render("├"+hbar+"┤") + "\n")
	for _, line := range lines {
		padded := padRight(line, cUsable)
		if lipgloss.Width(padded) > cUsable {
			padded = truncateWidth(padded, cUsable)
		}
		b.WriteString(bdr.Render("│") + innerSt.Render(" "+padded) + bdr.Render("│") + "\n")
	}
	b.WriteString(bdr.Render("└" + hbar + "┘"))
	return b.String()
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
