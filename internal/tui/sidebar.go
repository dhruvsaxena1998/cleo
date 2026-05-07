package tui

import (
	"fmt"
	"sort"
	"strings"

	"github.com/dhruvsaxena1998/cleo/internal/projects"
	"github.com/dhruvsaxena1998/cleo/internal/state"
)

func (m Model) renderSidebar(width int) string {
	var b strings.Builder
	b.WriteString(styleProject.Render("Projects") + "\n\n")
	if m.filter != "" {
		b.WriteString(styleDimmed.Render("/"+m.filter) + "\n")
	}
	projs := append([]projects.Project(nil), m.projects...)
	sort.Slice(projs, func(i, j int) bool { return projs[i].ID < projs[j].ID })
	for pi, p := range projs {
		if !m.matchesFilter(p.ID) && !m.projectHasMatching(p.ID) {
			continue
		}
		caret := "▶"
		if m.expanded[p.ID] {
			caret = "▼"
		}
		line := fmt.Sprintf("%s %s", caret, p.ID)
		if pi == m.cursor.projectIdx && m.cursor.agentIdx == -1 {
			line = styleSelected.Render(line)
		}
		b.WriteString(line + "\n")
		if !m.expanded[p.ID] {
			continue
		}
		ss := m.sessionsFor(p.ID)
		for ai, s := range ss {
			cfgAgent := m.ctx.Config.Agents[s.Agent]
			label := agentLabel(cfgAgent.Label, cfgAgent.Color)
			row := fmt.Sprintf("  %s %-20s %s %s", label, truncate(s.Name, 20), stateGlyph(string(s.State)), shortState(s.State))
			if pi == m.cursor.projectIdx && ai == m.cursor.agentIdx {
				row = styleSelected.Render(row)
			}
			b.WriteString(row + "\n")
		}
	}
	return b.String()
}

func (m Model) sessionsFor(pid string) []state.Session {
	var out []state.Session
	for _, s := range m.sessions {
		if s.ProjectID == pid && m.matchesFilter(s.ID, s.Name, s.Agent) {
			out = append(out, s)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].StartedAt.Before(out[j].StartedAt) })
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
	if len(s) <= n {
		return s
	}
	return s[:n-1] + "…"
}

func shortState(s state.State) string {
	switch s {
	case state.WaitingForInput:
		return "waiting"
	case state.Running:
		return "running"
	}
	return string(s)
}
