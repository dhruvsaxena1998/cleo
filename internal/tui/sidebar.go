package tui

import (
	"fmt"
	"sort"
	"strings"

	"github.com/charmbracelet/lipgloss"
	zone "github.com/lrstanley/bubblezone"

	"github.com/dhruvsaxena1998/cleo/internal/projects"
	"github.com/dhruvsaxena1998/cleo/internal/state"
)

// Zone id helpers for clickable tree rows. handleMouse reverses these to map a
// click back to a cursor position. Marking whole rows (not inner spans) keeps
// substring assertions in tests valid even on un-scanned output.
func projZoneID(pi int) string     { return fmt.Sprintf("tree-proj-%d", pi) }
func sessZoneID(pi, ai int) string { return fmt.Sprintf("tree-sess-%d-%d", pi, ai) }

// renderLeftColumn renders the full-height projects / sessions tree.
func (m Model) renderLeftColumn(w, h int) string {
	return m.renderTreePanel(w, h)
}

// renderSidebar is kept for the snapshot test (uses the tree content).
func (m Model) renderSidebar(width int) string {
	return m.renderTreeContent(width - 4)
}

// ── Tree panel ────────────────────────────────────────────────────────────────

func (m Model) renderTreePanel(w, h int) string {
	total := len(m.sessions)
	hint := fmt.Sprintf("%d sessions", total)
	contentH := h - 4
	if contentH < 1 {
		contentH = 1
	}
	allLines := strings.Split(m.renderTreeContent(w-4), "\n")
	scrollOff := m.cursorFlatIdx() - contentH + 1
	if scrollOff < 0 {
		scrollOff = 0
	}
	if scrollOff > len(allLines) {
		scrollOff = len(allLines)
	}
	return m.theme.PanelBox("Projects / Sessions", hint, allLines[scrollOff:], w, h)
}

func (m Model) renderTreeContent(innerW int) string {
	var b strings.Builder

	projs := m.visibleProjects()
	if len(projs) == 0 {
		b.WriteString(lipgloss.NewStyle().Foreground(m.theme.Overlay0).Render("no matching projects"))
		return b.String()
	}

	faint := lipgloss.NewStyle().Foreground(m.theme.Overlay0)
	dimmed := lipgloss.NewStyle().Foreground(m.theme.Subtext0)
	selectedSt := lipgloss.NewStyle().Background(m.theme.Surf0).Foreground(m.theme.Text).Bold(true)
	projectSt := lipgloss.NewStyle().Foreground(m.theme.Text).Bold(true)

	for pi, p := range projs {
		expanded := m.expanded[p.ID]
		ss := m.sessionsFor(p.ID)
		onProject := pi == m.cursor.projectIdx && m.cursor.agentIdx == -1

		active := 0
		for _, s := range ss {
			if !s.State.IsFinished() {
				active++
			}
		}

		caret := faint.Render("▸")
		if expanded {
			caret = dimmed.Render("▾")
		}

		countColor := m.theme.Subtext0
		if active > 0 {
			countColor = m.theme.Green
		}

		var projLine string
		if onProject {
			arrow := "▸"
			if expanded {
				arrow = "▾"
			}
			inner := fmt.Sprintf("%s %s", arrow, p.ID)
			projLine = selectedSt.Width(innerW).Render(inner)
		} else {
			name := projectSt.Render(truncateWidth(p.ID, innerW-6))
			countStr := lipgloss.NewStyle().Foreground(countColor).Render(fmt.Sprintf("%d", len(ss)))
			projLine = caret + " " + padRight(name, innerW-4) + countStr
		}
		b.WriteString(zone.Mark(projZoneID(pi), projLine) + "\n")

		if !expanded {
			continue
		}

		for ai, s := range ss {
			cfgAgent := m.ctx.Config.Agents[s.Agent]
			onAgent := pi == m.cursor.projectIdx && ai == m.cursor.agentIdx

			connector := "├"
			if ai == len(ss)-1 {
				connector = "└"
			}

			ageStr := sessionAge(s)
			shortSt := shortStateLabel(s.State)
			stColor := m.theme.StateColor(string(s.State))
			ageW := lipgloss.Width(ageStr)

			bracketed := "[" + cfgAgent.Label + "]"
			agentLbl := lipgloss.NewStyle().
				Foreground(lipgloss.Color(cfgAgent.Color)).Bold(true).
				Render(bracketed)
			labelW := len(bracketed)

			overhead := 1 + 1 + labelW + 1 + 4 + 1 + ageW
			nameW := innerW - overhead
			if nameW < 3 {
				nameW = 3
			}

			truncName := truncateWidth(s.Name, nameW)
			lPart := connector + " " + bracketed + " " + truncName
			rPart := fmt.Sprintf("%-4s", shortSt) + " " + ageStr
			gap := innerW - lipgloss.Width(lPart) - lipgloss.Width(rPart)
			if gap < 1 {
				gap = 1
			}

			var row string
			if onAgent {
				plain := lPart + strings.Repeat(" ", gap) + rPart
				row = selectedSt.Width(innerW).Render(plain)
			} else {
				stLabel := lipgloss.NewStyle().Foreground(stColor).Render(fmt.Sprintf("%-4s", shortSt))
				left := faint.Render(connector) + " " +
					agentLbl + " " +
					dimmed.Render(truncName)
				right := stLabel + " " + faint.Render(ageStr)
				row = left + strings.Repeat(" ", gap) + right
			}
			b.WriteString(zone.Mark(sessZoneID(pi, ai), row) + "\n")
		}
	}

	return strings.TrimRight(b.String(), "\n")
}

// ── Actions panel ─────────────────────────────────────────────────────────────

type panelAction struct {
	label   string
	key     string
	primary bool
}

func (m Model) renderActionsPanel(w, h int) string {
	_, hasSess := m.sessionAtCursor()

	var actions []panelAction
	if hasSess {
		actions = []panelAction{
			{"attach tmux session", "enter", true},
			{"open Project in editor", "ctrl+g", false},
			{"spawn sibling agent", "n", false},
			{"view session detail", "v", false},
			{"kill after confirm", "K", false},
		}
	} else {
		actions = []panelAction{
			{"spawn new agent", "n", true},
			{"open Project in editor", "ctrl+g", false},
			{"find sessions", "/", false},
			{"expand / collapse", "space", false},
			{"quit", "q", false},
		}
	}

	hint := "selected session"
	if !hasSess {
		hint = "no session selected"
	}

	cw := w - 4
	dimmed := lipgloss.NewStyle().Foreground(m.theme.Subtext0)
	bright := lipgloss.NewStyle().Foreground(m.theme.Text).Bold(true)
	lines := make([]string, 0, len(actions))
	for _, a := range actions {
		marker := "  "
		labelStyle := dimmed
		if a.primary {
			marker = lipgloss.NewStyle().Foreground(m.theme.Gold).Bold(true).Render("›") + " "
			labelStyle = bright
		}
		keyStr := lipgloss.NewStyle().Foreground(m.theme.Gold).Bold(true).Render(a.key)
		label := labelStyle.Render(a.label)
		gap := cw - lipgloss.Width(marker) - lipgloss.Width(label) - lipgloss.Width(keyStr) - 1
		if gap < 1 {
			gap = 1
		}
		lines = append(lines, marker+label+strings.Repeat(" ", gap)+keyStr)
	}

	return m.theme.PanelBox("Actions", hint, lines, w, h)
}

// ── Shared helpers ────────────────────────────────────────────────────────────

func (m Model) visibleProjects() []projects.Project {
	projs := append([]projects.Project(nil), m.projects...)
	sort.Slice(projs, func(i, j int) bool { return projs[i].ID < projs[j].ID })
	return projs
}

func (m Model) sessionsFor(pid string) []state.Session {
	var out []state.Session
	for _, s := range m.sessions {
		if s.ProjectID == pid {
			out = append(out, s)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if !out[i].StartedAt.Equal(out[j].StartedAt) {
			return out[i].StartedAt.Before(out[j].StartedAt)
		}
		return out[i].ID < out[j].ID // stable tiebreaker for sessions with equal/zero StartedAt
	})
	return out
}

func truncate(s string, n int) string {
	return truncateWidth(s, n)
}

func shortStateLabel(s state.State) string {
	switch s {
	case state.WaitingForInput:
		return "wait"
	case state.Running:
		return "run"
	case state.Idle:
		return "idle"
	case state.Spawning:
		return "spawn"
	case state.Completed:
		return "done"
	case state.Errored:
		return "err"
	case state.Dead:
		return "dead"
	}
	return string(s)
}

func shortState(s state.State) string {
	return shortStateLabel(s)
}

// cursorFlatIdx returns the 0-based row index of the cursor in the full
// rendered tree (one row per project header, one per expanded session).
func (m Model) cursorFlatIdx() int {
	projs := m.visibleProjects()
	row := 0
	for pi, p := range projs {
		if pi == m.cursor.projectIdx {
			if m.cursor.agentIdx < 0 {
				return row // on the project header
			}
			return row + 1 + m.cursor.agentIdx // +1 for the header row
		}
		row++ // project header
		if m.expanded[p.ID] {
			row += len(m.sessionsFor(p.ID))
		}
	}
	return 0
}

// sessionAge returns the age string for a session row (last event or start).
func sessionAge(s state.Session) string {
	if !s.LastEventAt.IsZero() {
		return humanDuration(s.LastEventAt)
	}
	return humanDuration(s.StartedAt)
}
