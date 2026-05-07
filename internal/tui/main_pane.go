package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/dhruvsaxena1998/cleo/internal/events"
	"github.com/dhruvsaxena1998/cleo/internal/state"
)

func (m Model) renderMain(width int) string {
	sess, ok := m.selectedSession()
	if !ok {
		return styleDimmed.Render("press v on an agent to view")
	}
	header := fmt.Sprintf("%s\n%s  started %s ago  tools: %d  last: %s",
		sess.ID, stateGlyph(string(sess.State)),
		humanDuration(sess.StartedAt), sess.ToolCount, humanDuration(sess.LastEventAt))

	log := m.ctx.Events(sess.ID)
	entries, _ := log.Tail(m.ctx.Config.UI.EventLogLines)
	var lines []string
	for _, e := range entries {
		lines = append(lines, formatEntry(e))
	}
	eventsBlock := strings.Join(lines, "\n")

	pane := m.paneCache[sess.ID]
	return strings.Join([]string{
		header,
		"── recent ─────",
		eventsBlock,
		"── pane preview ─────",
		truncateLines(pane, m.ctx.Config.UI.PanePreviewLines),
	}, "\n")
}

func (m Model) selectedSession() (state.Session, bool) {
	if m.selected == "" {
		return state.Session{}, false
	}
	for _, s := range m.sessions {
		if s.ID == m.selected {
			return s, true
		}
	}
	return state.Session{}, false
}

func formatEntry(e events.Entry) string {
	return fmt.Sprintf("%s  %s  %s", e.At.Format("15:04:05"), e.Type, e.Tool)
}

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
		return "just now"
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
