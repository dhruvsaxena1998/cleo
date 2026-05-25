package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
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
	cw := w - 4
	col := cw / 6
	if col < 6 {
		col = 6
	}

	faint := lipgloss.NewStyle().Foreground(m.theme.Overlay0)
	bright := lipgloss.NewStyle().Foreground(m.theme.Text).Bold(true)
	metric := lipgloss.NewStyle().Foreground(m.theme.Accent).Bold(true)
	dimmed := lipgloss.NewStyle().Foreground(m.theme.Subtext0)

	if !has {
		labels := faint.Render(
			padRight("agent", col) + padRight("state", col) + padRight("project", col) +
				padRight("runtime", col) + padRight("tools", col) + "last")
		values := faint.Render("─  no session selected  ─")
		return m.theme.PanelBox("Session", "—", []string{labels, values}, w, h)
	}

	cfgAgent := m.ctx.Config.Agents[sess.Agent]
	badge := m.theme.AgentBadge(cfgAgent.Label, cfgAgent.Color) + " " + dimmed.Render(sess.Agent)

	labelRow := faint.Render(padRight("agent", col)) +
		faint.Render(padRight("state", col)) +
		faint.Render(padRight("project", col)) +
		faint.Render(padRight("runtime", col)) +
		faint.Render(padRight("tools", col)) +
		faint.Render("last")

	stateVal := lipgloss.NewStyle().Foreground(m.theme.StateColor(string(sess.State))).Bold(true).Render(
		truncateWidth(string(sess.State), col-1))

	valueRow := padRight(badge, col) +
		padRight(stateVal, col) +
		padRight(bright.Render(truncateWidth(sess.ProjectID, col-1)), col) +
		padRight(metric.Render(sinceLabel(sess.StartedAt)), col) +
		padRight(metric.Render(fmt.Sprintf("%d", sess.ToolCount)), col) +
		metric.Render(humanDuration(sess.LastEventAt))

	hint := truncateWidth(sess.ID, w-14)
	return m.theme.PanelBox("Session", hint, []string{labelRow, valueRow}, w, h)
}

// ── Events panel ──────────────────────────────────────────────────────────────

func (m Model) renderEventsPanel(w, h int, sess state.Session, has bool) string {
	contentH := h - 4
	if contentH < 1 {
		contentH = 1
	}

	faint := lipgloss.NewStyle().Foreground(m.theme.Overlay0)
	var hint, lines string
	if !has {
		hint = "—"
		lines = faint.Render("select a session to view events")
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
			b.WriteString(m.theme.FormatEventRow(e, w-4, isLast) + "\n")
		}
		lines = strings.TrimRight(b.String(), "\n")
	}

	return m.theme.PanelBox("Events", hint, strings.Split(lines, "\n"), w, h)
}

// ── Preview panel ─────────────────────────────────────────────────────────────

func (m Model) renderPreviewPanel(w, h int, sess state.Session, has bool) string {
	contentH := h - 4
	if contentH < 1 {
		contentH = 1
	}

	faint := lipgloss.NewStyle().Foreground(m.theme.Overlay0)

	if !has {
		return m.theme.PanelBox("Terminal Preview", "tmux capture-pane -p",
			[]string{faint.Render("navigate to a session to view its terminal")}, w, h)
	}

	if sess.State.IsFinished() {
		return m.theme.PanelBox("Terminal Preview", "session finished",
			[]string{faint.Render("tmux session is gone; press K to remove this record")}, w, h)
	}

	pane := m.paneCache[sess.ID]
	hint := "tmux capture-pane -p"
	switch {
	case pane == "":
		return m.theme.PanelBox("Terminal Preview", hint,
			[]string{faint.Render("loading…  press v to refresh")}, w, h)
	case strings.TrimSpace(pane) == "":
		return m.theme.PanelBox("Terminal Preview", hint,
			[]string{faint.Render("agent hasn't rendered yet — press Enter to attach")}, w, h)
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
		// Truncate with ANSI-awareness so escape sequences aren't sliced
		// mid-code. Raw ANSI passes through to preserve agent output colors.
		body[i] = ansi.Truncate(l, w-4, "")
	}
	return m.theme.PanelBox("Terminal Preview", hint, body, w, h)
}

// ── Fallback text views (used by snapshot test & renderMain) ──────────────────

func (m Model) renderDashboard(width int) string {
	faint := lipgloss.NewStyle().Foreground(m.theme.Overlay0)
	bright := lipgloss.NewStyle().Foreground(m.theme.Text).Bold(true)
	dimmed := lipgloss.NewStyle().Foreground(m.theme.Subtext0)
	idSt := lipgloss.NewStyle().Foreground(m.theme.Accent).Bold(true)

	if len(m.sessions) == 0 {
		return "\n\n  " + bright.Render("No sessions yet") + "\n\n" +
			"  " + dimmed.Render("Register projects with cleo add, then press n to spawn an agent.") + "\n\n" +
			"  " + m.theme.KeyHint("n", "new agent") + "   " + m.theme.KeyHint("/", "filter projects")
	}
	var b strings.Builder
	stats := m.sessionStats()
	b.WriteString("  " + faint.Render("overview") + "\n")
	b.WriteString("  " +
		m.theme.Pill(fmt.Sprintf("%d sessions", len(m.sessions)), m.theme.Subtext0) + " " +
		m.theme.Pill(fmt.Sprintf("%d live", stats.live), m.theme.Green) + " " +
		m.theme.Pill(fmt.Sprintf("%d waiting", stats.waiting), m.theme.Peach) + "\n\n")
	for _, s := range m.sessions {
		cfgAgent := m.ctx.Config.Agents[s.Agent]
		badge := agentLabel(cfgAgent.Label, cfgAgent.Color)
		b.WriteString(fmt.Sprintf("  %s  %s  %s  %s  %s\n",
			badge,
			padRight(idSt.Render(truncateWidth(s.ID, 30)), 30),
			m.theme.StyledGlyph(string(s.State)),
			padRight(m.theme.StyledStateText(string(s.State)), 18),
			dimmed.Render(humanDuration(s.StartedAt)),
		))
	}
	b.WriteString("\n  " + faint.Render("Select a session, then press v for logs or enter to attach."))
	return b.String()
}

func (m Model) renderSessionDetail(sess state.Session, width int) string {
	faint := lipgloss.NewStyle().Foreground(m.theme.Overlay0)
	dimmed := lipgloss.NewStyle().Foreground(m.theme.Subtext0)
	idSt := lipgloss.NewStyle().Foreground(m.theme.Accent).Bold(true)
	metric := lipgloss.NewStyle().Foreground(m.theme.Accent).Bold(true)

	var b strings.Builder
	b.WriteString("  " + faint.Render("session") + "\n")
	b.WriteString("  " + idSt.Render(truncateWidth(sess.ID, width-4)) + "\n\n")
	dot := faint.Render("  ·  ")
	statsLine := m.theme.StyledGlyph(string(sess.State)) + "  " + m.theme.StyledStateText(string(sess.State)) +
		dot + dimmed.Render("started ") + metric.Render(sinceLabel(sess.StartedAt)) +
		dot + dimmed.Render("tools ") + metric.Render(fmt.Sprintf("%d", sess.ToolCount)) +
		dot + dimmed.Render("last ") + metric.Render(humanDuration(sess.LastEventAt))
	b.WriteString("  " + truncateWidth(statsLine, width-4) + "\n")
	if sess.LastMessage != "" {
		b.WriteString("  " + dimmed.Render(truncateWidth(sess.LastMessage, width-4)) + "\n")
	}
	b.WriteString("\n  " + m.theme.SectionDivider("events", width-4) + "\n")
	log := m.ctx.Events(sess.ID)
	entries, _ := log.Tail(m.ctx.Config.UI.EventLogLines)
	if len(entries) == 0 {
		b.WriteString("  " + faint.Render("no events yet") + "\n")
	} else {
		start := len(entries) - 9
		if start < 0 {
			start = 0
		}
		for i, e := range entries[start:] {
			isLast := i == len(entries[start:])-1
			b.WriteString("  " + m.theme.FormatEventRow(e, width-4, isLast) + "\n")
		}
	}
	b.WriteString("\n  " + m.theme.SectionDivider("terminal", width-4) + "\n")
	pane := m.paneCache[sess.ID]
	if pane == "" {
		b.WriteString("  " + faint.Render("press v to load preview") + "\n")
	} else {
		for _, line := range strings.Split(truncateLines(pane, 12), "\n") {
			b.WriteString("  " + dimmed.Render(truncateWidth(line, width-4)) + "\n")
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
