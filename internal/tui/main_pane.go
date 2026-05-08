package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/dhruvsaxena1998/cleo/internal/events"
	"github.com/dhruvsaxena1998/cleo/internal/state"
)

// renderRightColumn assembles the three-panel right stack:
// ┌ Session metadata ┐  (fixed height — 6-cell meta grid)
// ├ Events log       ┤  (~28% — compact log strip)
// └ Terminal preview ┘  (remainder — largest panel)
func (m Model) renderRightColumn(w, h int) string {
	const metaH = 6 // border(2) + title(1) + sep(1) + labels(1) + values(1)
	eventsH := h * 28 / 100
	if eventsH < 6 {
		eventsH = 6
	}
	previewH := h - metaH - eventsH
	if previewH < 5 {
		previewH = 5
	}

	sess, hasSess := m.selectedSession()

	meta := m.renderMetaPanel(w, metaH, sess, hasSess)
	ev := m.renderEventsPanel(w, eventsH, sess, hasSess)
	preview := m.renderPreviewPanel(w, previewH, sess, hasSess)
	return meta + "\n" + ev + "\n" + preview
}

// renderMain is kept for the snapshot test.
func (m Model) renderMain(width int) string {
	sess, ok := m.selectedSession()
	if !ok {
		return m.renderDashboard(width)
	}
	return m.renderSessionDetail(sess, width)
}

// ── Meta panel (6-cell grid) ──────────────────────────────────────────────────

func (m Model) renderMetaPanel(w, h int, sess state.Session, has bool) string {
	cw := w - 4 // usable content width
	col := cw / 6
	if col < 6 {
		col = 6
	}

	labelStyle := styleFaint
	valueStyle := styleBright

	if !has {
		labels := labelStyle.Render(
			padRight("agent", col) + padRight("state", col) + padRight("project", col) +
				padRight("runtime", col) + padRight("tools", col) + "last")
		values := styleFaint.Render("─  no session selected  ─")
		return panelBox("Session", "—", []string{labels, values}, w, h)
	}

	cfgAgent := m.ctx.Config.Agents[sess.Agent]
	badge := agentBadge(cfgAgent.Label, cfgAgent.Color) + " " + styleDimmed.Render(sess.Agent)

	labelRow := labelStyle.Render(padRight("agent", col)) +
		labelStyle.Render(padRight("state", col)) +
		labelStyle.Render(padRight("project", col)) +
		labelStyle.Render(padRight("runtime", col)) +
		labelStyle.Render(padRight("tools", col)) +
		labelStyle.Render("last")

	stateVal := lipgloss.NewStyle().Foreground(stateColor(string(sess.State))).Bold(true).Render(
		truncateWidth(string(sess.State), col-1))

	valueRow := padRight(badge, col) +
		padRight(stateVal, col) +
		padRight(valueStyle.Render(truncateWidth(sess.ProjectID, col-1)), col) +
		padRight(styleMetric.Render(sinceLabel(sess.StartedAt)), col) +
		padRight(styleMetric.Render(fmt.Sprintf("%d", sess.ToolCount)), col) +
		styleMetric.Render(humanDuration(sess.LastEventAt))

	hint := truncateWidth(sess.ID, w-14)
	return panelBox("Session", hint, []string{labelRow, valueRow}, w, h)
}

// ── Events panel ──────────────────────────────────────────────────────────────

func (m Model) renderEventsPanel(w, h int, sess state.Session, has bool) string {
	contentH := h - 4
	if contentH < 1 {
		contentH = 1
	}

	var hint, lines string
	if !has {
		hint = "—"
		lines = styleFaint.Render("select a session to view events")
	} else {
		log := m.ctx.Events(sess.ID)
		entries, _ := log.Tail(m.ctx.Config.UI.EventLogLines)
		hint = fmt.Sprintf("tail events/%s.jsonl", sess.ID)

		var b strings.Builder
		start := len(entries) - contentH
		if start < 0 {
			start = 0
		}
		for i, e := range entries[start:] {
			isLast := i == len(entries[start:])-1
			b.WriteString(formatEventRow(e, w-4, isLast) + "\n")
		}
		lines = strings.TrimRight(b.String(), "\n")
	}

	return panelBox("Events", hint, strings.Split(lines, "\n"), w, h)
}

// ── Preview panel ─────────────────────────────────────────────────────────────

func (m Model) renderPreviewPanel(w, h int, sess state.Session, has bool) string {
	contentH := h - 4
	if contentH < 1 {
		contentH = 1
	}

	if !has {
		return panelBox("Terminal Preview", "tmux capture-pane -p",
			[]string{styleFaint.Render("navigate to a session to view its terminal")}, w, h)
	}

	if sess.State.IsFinished() {
		return panelBox("Terminal Preview", "session finished",
			[]string{styleFaint.Render("tmux session is gone; press K to remove this record")}, w, h)
	}

	pane := m.paneCache[sess.ID]
	hint := "tmux capture-pane -p"
	if pane == "" {
		return panelBox("Terminal Preview", hint,
			[]string{styleFaint.Render("loading…  press v to refresh")}, w, h)
	}

	allLines := strings.Split(pane, "\n")

	// Strip trailing blank lines — full-screen TUIs pad the pane to terminal
	// height, so the captured content ends with many empty rows.
	for len(allLines) > 1 && strings.TrimSpace(allLines[len(allLines)-1]) == "" {
		allLines = allLines[:len(allLines)-1]
	}

	// Show the last contentH lines of meaningful content.
	start := len(allLines) - contentH
	if start < 0 {
		start = 0
	}
	shown := allLines[start:]
	body := make([]string, len(shown))
	for i, l := range shown {
		body[i] = styleDimmed.Render(l)
	}
	return panelBox("Terminal Preview", hint, body, w, h)
}

// ── Fallback text views (used by snapshot test & renderMain) ──────────────────

func (m Model) renderDashboard(width int) string {
	if len(m.sessions) == 0 {
		return "\n\n  " + styleBright.Render("No sessions yet") + "\n\n" +
			"  " + styleDimmed.Render("Register projects with cleo add, then press n to spawn an agent.") + "\n\n" +
			"  " + keyHint("n", "new agent") + "   " + keyHint("/", "filter projects")
	}
	var b strings.Builder
	stats := m.sessionStats()
	b.WriteString("  " + styleFaint.Render("overview") + "\n")
	b.WriteString("  " +
		pill(fmt.Sprintf("%d sessions", len(m.sessions)), clrSubtext) + " " +
		pill(fmt.Sprintf("%d live", stats.live), clrGreen) + " " +
		pill(fmt.Sprintf("%d waiting", stats.waiting), clrAmber) + "\n\n")
	for _, s := range m.sessions {
		cfgAgent := m.ctx.Config.Agents[s.Agent]
		badge := agentLabel(cfgAgent.Label, cfgAgent.Color)
		b.WriteString(fmt.Sprintf("  %s  %s  %s  %s  %s\n",
			badge,
			padRight(styleID.Render(truncateWidth(s.ID, 30)), 30),
			styledGlyph(string(s.State)),
			padRight(styledStateText(string(s.State)), 18),
			styleDimmed.Render(humanDuration(s.StartedAt)),
		))
	}
	b.WriteString("\n  " + styleFaint.Render("Select a session, then press v for logs or enter to attach."))
	return b.String()
}

func (m Model) renderSessionDetail(sess state.Session, width int) string {
	var b strings.Builder
	b.WriteString("  " + styleFaint.Render("session") + "\n")
	b.WriteString("  " + styleID.Render(truncateWidth(sess.ID, width-4)) + "\n\n")
	dot := styleFaint.Render("  ·  ")
	statsLine := styledGlyph(string(sess.State)) + "  " + styledStateText(string(sess.State)) +
		dot + styleDimmed.Render("started ") + styleMetric.Render(sinceLabel(sess.StartedAt)) +
		dot + styleDimmed.Render("tools ") + styleMetric.Render(fmt.Sprintf("%d", sess.ToolCount)) +
		dot + styleDimmed.Render("last ") + styleMetric.Render(humanDuration(sess.LastEventAt))
	b.WriteString("  " + truncateWidth(statsLine, width-4) + "\n")
	if sess.LastMessage != "" {
		b.WriteString("  " + styleDimmed.Render(truncateWidth(sess.LastMessage, width-4)) + "\n")
	}
	b.WriteString("\n  " + sectionDivider("events", width-4) + "\n")
	log := m.ctx.Events(sess.ID)
	entries, _ := log.Tail(m.ctx.Config.UI.EventLogLines)
	if len(entries) == 0 {
		b.WriteString("  " + styleFaint.Render("no events yet") + "\n")
	} else {
		start := len(entries) - 9
		if start < 0 {
			start = 0
		}
		for i, e := range entries[start:] {
			isLast := i == len(entries[start:])-1
			b.WriteString("  " + formatEventRow(e, width-4, isLast) + "\n")
		}
	}
	b.WriteString("\n  " + sectionDivider("terminal", width-4) + "\n")
	pane := m.paneCache[sess.ID]
	if pane == "" {
		b.WriteString("  " + styleFaint.Render("press v to load preview") + "\n")
	} else {
		for _, line := range strings.Split(truncateLines(pane, 12), "\n") {
			b.WriteString("  " + styleDimmed.Render(truncateWidth(line, width-4)) + "\n")
		}
	}
	return b.String()
}

func (m Model) selectedSession() (state.Session, bool) {
	// Cursor always wins — m.selected is only used to pin the right panel
	// when no cursor session exists (e.g. project row selected).
	if sess, ok := m.sessionAtCursor(); ok {
		return sess, true
	}
	if m.selected != "" {
		for _, s := range m.sessions {
			if s.ID == m.selected {
				return s, true
			}
		}
	}
	return state.Session{}, false
}

// ── Event row formatting ──────────────────────────────────────────────────────

func eventTypeColor(t string) lipgloss.Color {
	switch t {
	case "PreToolUse", "pre_tool_use":
		return clrAmber
	case "PostToolUse", "post_tool_use":
		return clrGreen
	case "Stop", "stop", "SessionEnd", "session_end", "idle_timeout":
		return clrOrange
	case "Notification", "notification", "user_resume":
		return clrAccent
	case "SessionStart", "session_start":
		return clrAccent
	case "error", "dead":
		return clrRed
	}
	return clrSubtext
}

func formatEventRow(e events.Entry, width int, highlight bool) string {
	// Columns: time(9)  type(16)  detail(flex)  duration(8)
	ts := styleFaint.Render(e.At.Format("15:04:05"))
	evType := lipgloss.NewStyle().Foreground(eventTypeColor(e.Type)).Render(fmt.Sprintf("%-16s", e.Type))
	detail := ""
	if e.Detail != "" {
		detail = e.Detail
	} else if e.Tool != "" {
		detail = e.Tool
	}
	dur := ""
	if e.DurationS > 0 {
		dur = styleFaint.Render(fmt.Sprintf("%.1fs", e.DurationS))
	}

	durW := 6
	fixed := 9 + 2 + 16 + 2 + durW + 2
	detailW := width - fixed
	if detailW < 4 {
		detailW = 4
	}
	detailStr := styleDimmed.Render(truncateWidth(detail, detailW))
	if e.Type == "Notification" || e.Type == "notification" {
		detailStr = lipgloss.NewStyle().Foreground(clrGold).Render(truncateWidth(detail, detailW))
	}

	row := ts + "  " + evType + "  " + padRight(detailStr, detailW) + "  " + padRight(dur, durW)
	if highlight {
		row = styleSelected.Render(row)
	}
	return row
}

// ── Time helpers ──────────────────────────────────────────────────────────────

func truncateLines(s string, n int) string {
	lines := strings.Split(s, "\n")
	if len(lines) <= n {
		return s
	}
	return strings.Join(lines[len(lines)-n:], "\n")
}

func humanDuration(t time.Time) string {
	if t.IsZero() {
		return "—"
	}
	d := time.Since(t)
	switch {
	case d < time.Second:
		return "now"
	case d < time.Minute:
		return fmt.Sprintf("%ds", int(d.Seconds()))
	case d < time.Hour:
		return fmt.Sprintf("%dm", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh", int(d.Hours()))
	default:
		return fmt.Sprintf("%dd", int(d.Hours()/24))
	}
}

func sinceLabel(t time.Time) string {
	v := humanDuration(t)
	if v == "—" || v == "now" {
		return v
	}
	return v + " ago"
}
