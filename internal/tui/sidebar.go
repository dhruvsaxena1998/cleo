package tui

import (
	"fmt"
	"sort"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/dhruvsaxena1998/cleo/internal/projects"
	"github.com/dhruvsaxena1998/cleo/internal/state"
)

// renderLeftColumn assembles the two-panel left stack:
// ┌ Filter ┐  (fixed height)
// └ Tree   ┘  (flexible)
func (m Model) renderLeftColumn(w, h int) string {
	const filterH = 6 // border(2) + title(1) + sep(1) + 2 content rows — matches metaH on the right
	treeH := h - filterH
	if treeH < 5 {
		treeH = 5
	}

	filter := m.renderFilterPanel(w, filterH)
	tree := m.renderTreePanel(w, treeH)
	return filter + "\n" + tree
}

// renderSidebar is kept for the snapshot test (uses the tree content).
func (m Model) renderSidebar(width int) string {
	return m.renderTreeContent(width - 4)
}

// ── Filter panel ──────────────────────────────────────────────────────────────

func (m Model) renderFilterPanel(w, h int) string {
	faint := lipgloss.NewStyle().Foreground(m.theme.Overlay0)
	hint := "inactive"
	if m.mode == ModeFilter {
		hint = "active"
	} else if m.filter != "" {
		hint = m.filter
	}

	var line string
	if m.filter != "" || m.mode == ModeFilter {
		cur := m.filter
		if m.mode == ModeFilter {
			cur = m.filter + "▌"
		}
		line = lipgloss.NewStyle().Foreground(m.theme.Gold).Bold(true).Render("/") + " " +
			lipgloss.NewStyle().Foreground(m.theme.Text).Bold(true).Render(cur)
	} else {
		line = faint.Render("/ type to filter sessions and projects")
	}

	return m.theme.PanelBox("Filter", hint, []string{line, ""}, w, h)
}

// ── Tree panel ────────────────────────────────────────────────────────────────

func (m Model) renderTreePanel(w, h int) string {
	total := len(m.sessions)
	stats := m.sessionStats()
	hint := fmt.Sprintf("%d sessions", total)
	lines := m.renderTreeContent(w - 4)
	return m.theme.PanelBox("Projects / Sessions", hint, splitLines(lines, stats.live, stats.waiting), w, h)
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
		b.WriteString(projLine + "\n")

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
			b.WriteString(row + "\n")
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
			{"spawn sibling agent", "n", false},
			{"view session detail", "v", false},
			{"kill after confirm", "K", false},
		}
	} else {
		actions = []panelAction{
			{"spawn new agent", "n", true},
			{"filter sessions", "/", false},
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
	var out []projects.Project
	for _, p := range projs {
		if m.matchesFilter(p.ID) || m.projectHasMatching(p.ID) {
			out = append(out, p)
		}
	}
	return out
}

func (m Model) sessionsFor(pid string) []state.Session {
	var out []state.Session
	for _, s := range m.sessions {
		if s.ProjectID == pid && m.matchesFilter(s.ID, s.Name, s.Agent) {
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

func (m Model) matchesFilter(parts ...string) bool {
	if m.filter == "" {
		return true
	}
	for _, p := range parts {
		if strings.Contains(strings.ToLower(p), strings.ToLower(m.filter)) {
			return true
		}
	}
	return false
}

func (m Model) projectHasMatching(pid string) bool {
	for _, s := range m.sessions {
		if s.ProjectID == pid && m.matchesFilter(s.ID, s.Name, s.Agent) {
			return true
		}
	}
	return false
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

// splitLines converts a multi-line string to a slice.
func splitLines(s string, _, _ int) []string {
	return strings.Split(s, "\n")
}

// sessionAge returns the age string for a session row (last event or start).
func sessionAge(s state.Session) string {
	if !s.LastEventAt.IsZero() {
		return humanDuration(s.LastEventAt)
	}
	return humanDuration(s.StartedAt)
}
