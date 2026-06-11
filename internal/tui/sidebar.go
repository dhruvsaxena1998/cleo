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
	return m.theme.PanelBox(withIcon(m.theme.Icons.Project, "Projects / Sessions"), hint, allLines[scrollOff:], w, h)
}

func (m Model) renderTreeContent(innerW int) string {
	var b strings.Builder

	projs := m.visibleProjects()
	if len(projs) == 0 {
		b.WriteString(lipgloss.NewStyle().Foreground(m.theme.Overlay0).Render("no matching projects"))
		return b.String()
	}

	ic := m.theme.Icons
	faint := lipgloss.NewStyle().Foreground(m.theme.Overlay0)
	dimmed := lipgloss.NewStyle().Foreground(m.theme.Subtext0)
	selectedSt := lipgloss.NewStyle().Background(m.theme.Surf0).Foreground(m.theme.Text).Bold(true)
	projectSt := lipgloss.NewStyle().Foreground(m.theme.Text).Bold(true)
	// barSt paints the accent gutter that marks the cursor row. It sits on the
	// Surf0 highlight so the bar reads as part of the selected band, not a stray
	// glyph in the margin. Selected rows render plain text under selectedSt; the
	// per-element colours (agent, state) intentionally give way to the uniform
	// highlight, matching the pre-overhaul selection look.
	barSt := lipgloss.NewStyle().Foreground(m.theme.Accent).Background(m.theme.Surf0)
	bar := barSt.Render("▎")

	for pi, p := range projs {
		expanded := m.expanded[p.ID]
		ss := m.sessionsFor(p.ID)
		onProject := m.cursor.onProject(pi)

		active := 0
		for _, s := range ss {
			if !s.State.IsFinished() {
				active++
			}
		}

		folder := ic.FolderClosed
		if expanded {
			folder = ic.FolderOpen
		}

		countColor := m.theme.Subtext0
		if active > 0 {
			countColor = m.theme.Green
		}
		countStr := lipgloss.NewStyle().Foreground(countColor).Render(fmt.Sprintf("%d", len(ss)))

		var projLine string
		if onProject {
			// bar(1) + highlighted band(innerW-1); the folder hugs the bar, so the
			// folder column lines up with the unselected rows' " "+folder gutter.
			inner := withIcon(folder, truncateWidth(p.ID, innerW-4))
			projLine = bar + selectedSt.Width(innerW-1).Render(inner)
		} else {
			left := " " + withIcon(dimmed.Render(folder), projectSt.Render(truncateWidth(p.ID, innerW-7)))
			gap := innerW - lipgloss.Width(left) - lipgloss.Width(countStr)
			if gap < 1 {
				gap = 1
			}
			projLine = left + strings.Repeat(" ", gap) + countStr
		}
		b.WriteString(zone.Mark(projZoneID(pi), projLine) + "\n")

		if !expanded {
			continue
		}

		for ai, s := range ss {
			cfgAgent := m.ctx.Config.Agents[s.Agent]
			onAgent := m.cursor.onAgent(pi, ai)

			connector := "├"
			if ai == len(ss)-1 {
				connector = "└"
			}

			ageStr := sessionAge(s)
			shortSt := fmt.Sprintf("%-4s", shortStateLabel(s.State))
			stColor := m.theme.StateColor(string(s.State))
			glyph := m.theme.stateGlyph(string(s.State))
			label := cfgAgent.Label
			name := s.Name
			if s.HasWorktree() {
				// Worktree badge: the session runs in an isolated git worktree.
				name = worktreeBadge(ic) + " " + name
			}

			// Left fixed cells before the name: gutter(1) connector(1) space(1)
			// glyph(1) withIcon-gap(2) label space(1); right cells: shortSt(4)
			// space(1) age.
			rightW := lipgloss.Width(shortSt) + 1 + lipgloss.Width(ageStr)
			nameW := innerW - (7 + lipgloss.Width(label)) - rightW - 1
			if nameW < 3 {
				nameW = 3
			}
			truncName := truncateWidth(name, nameW)

			var row string
			if onAgent {
				lInner := connector + " " + withIcon(glyph, label) + " " + truncName
				rInner := shortSt + " " + ageStr
				gap := (innerW - 1) - lipgloss.Width(lInner) - lipgloss.Width(rInner)
				if gap < 1 {
					gap = 1
				}
				inner := lInner + strings.Repeat(" ", gap) + rInner
				row = bar + selectedSt.Width(innerW-1).Render(inner)
			} else {
				// The marker pulses for working sessions (pulseColor); the state
				// label keeps the static colour so only the dot breathes.
				glyphSt := lipgloss.NewStyle().Foreground(m.pulseColor(string(s.State))).Render(glyph)
				labelSt := lipgloss.NewStyle().Foreground(lipgloss.Color(cfgAgent.Color)).Bold(true).Render(label)
				left := " " + faint.Render(connector) + " " + withIcon(glyphSt, labelSt) + " " + dimmed.Render(truncName)
				right := lipgloss.NewStyle().Foreground(stColor).Render(shortSt) + " " + faint.Render(ageStr)
				gap := innerW - lipgloss.Width(left) - lipgloss.Width(right)
				if gap < 1 {
					gap = 1
				}
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

	return m.theme.PanelBox(withIcon(m.theme.Icons.Tool, "Actions"), hint, lines, w, h)
}

// ── Shared helpers ────────────────────────────────────────────────────────────

func (m Model) visibleProjects() []projects.Project {
	projs := append([]projects.Project(nil), m.projects...)
	sort.Slice(projs, func(i, j int) bool { return projs[i].ID < projs[j].ID })
	return projs
}

// treeShape captures the current sidebar shape (visible projects, their
// expanded flags, and session counts) for the cursor to navigate over. It is
// the single bridge from Model data to the positional logic in cursor.go.
func (m Model) treeShape() treeShape {
	projs := m.visibleProjects()
	rows := make([]projectRowShape, len(projs))
	for i, p := range projs {
		rows[i] = projectRowShape{
			expanded: m.expanded[p.ID],
			sessions: len(m.sessionsFor(p.ID)),
		}
	}
	return treeShape{rows: rows}
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
	return m.cursor.flatRow(m.treeShape())
}

// sessionAge returns the age string for a session row (last event or start).
func sessionAge(s state.Session) string {
	if !s.LastEventAt.IsZero() {
		return humanDuration(s.LastEventAt)
	}
	return humanDuration(s.StartedAt)
}
