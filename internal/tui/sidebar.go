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
			if s.State != state.Dead {
				active++
			}
		}

		// compute alert badge for this project
		var needsInputN, workingN int
		for _, s := range ss {
			switch ToDisplayState(s.State) {
			case DisplayNeedsInput:
				needsInputN++
			case DisplayWorking:
				workingN++
			}
		}

		var borderChar, badgeStr string
		if needsInputN > 0 {
			borderChar = lipgloss.NewStyle().Foreground(m.theme.Gold).Render("│")
			badgeStr = " " + lipgloss.NewStyle().
				Background(m.theme.Gold).
				Foreground(lipgloss.Color("#11111b")).
				Bold(true).
				Padding(0, 1).
				Render(fmt.Sprintf("⚠ %d input", needsInputN))
		} else if workingN > 0 {
			borderChar = lipgloss.NewStyle().Foreground(m.theme.Blue).Render("│")
			badgeStr = " " + lipgloss.NewStyle().
				Foreground(m.theme.Blue).
				Render(fmt.Sprintf("◉ %d work", workingN))
		} else {
			borderChar = lipgloss.NewStyle().Foreground(m.theme.Overlay0).Render("│")
		}

		countColor := m.theme.Subtext0
		if active > 0 {
			countColor = m.theme.Green
		}

		var projLine string
		if onProject {
			arrow := "▶"
			if expanded {
				arrow = "▼"
			}
			inner := fmt.Sprintf("%s %s", arrow, p.ID)
			projLine = borderChar + selectedSt.Width(innerW-1).Render(inner)
		} else {
			caret := faint.Render("▶")
			if expanded {
				caret = dimmed.Render("▼")
			}
			badgeW := lipgloss.Width(badgeStr)
			nameAvail := innerW - 1 - 2 - badgeW - 3
			if nameAvail < 4 {
				nameAvail = 4
			}
			name := projectSt.Render(truncateWidth(p.ID, nameAvail))
			countStr := lipgloss.NewStyle().Foreground(countColor).Render(fmt.Sprintf("%d", len(ss)))
			projLine = borderChar + " " + caret + " " + name + badgeStr + " " + countStr
		}
		b.WriteString(projLine + "\n")

		if !expanded {
			continue
		}

		for ai, s := range ss {
			cfgAgent := m.ctx.Config.Agents[s.Agent]
			onAgent := pi == m.cursor.projectIdx && ai == m.cursor.agentIdx
			ds := ToDisplayState(s.State)

			// agent badge text: icon if set, else label
			badgeText := cfgAgent.Label
			if cfgAgent.Icon != "" {
				badgeText = cfgAgent.Icon
			}
			bracketed := "[" + badgeText + "]"
			badgeW := len(bracketed)

			// right-side fixed parts
			ageStr := sessionAge(s)
			ageW := lipgloss.Width(ageStr)
			toolStr := ""
			toolW := 0
			if s.ToolCount > 0 {
				toolStr = fmt.Sprintf("⚒ %d", s.ToolCount)
				toolW = lipgloss.Width(toolStr) + 1 // +1 for space before
			}

			// compute available width for name + message
			// overhead: indent(2) + glyph(1) + sp(1) + badge + sp(1) + sp(1) + toolW + ageW
			overhead := 2 + 1 + 1 + badgeW + 1 + 1 + toolW + ageW
			avail := innerW - overhead
			if avail < 4 {
				avail = 4
			}
			nameW := avail * 40 / 100
			if nameW < 6 {
				nameW = avail
			}
			msgW := avail - nameW - 1
			if msgW < 0 {
				msgW = 0
			}

			truncName := truncateWidth(s.Name, nameW)
			truncMsg := ""
			if msgW > 2 && s.LastMessage != "" {
				truncMsg = truncateWidth(s.LastMessage, msgW)
			}

			// plain (unstyled) for selected-row rendering
			plainGlyph := displayStateGlyph(ds)
			plainLeft := "  " + plainGlyph + " " + bracketed + " " + truncName
			if truncMsg != "" {
				plainLeft += " " + truncMsg
			}
			var rParts []string
			if toolStr != "" {
				rParts = append(rParts, toolStr)
			}
			rParts = append(rParts, ageStr)
			plainRight := strings.Join(rParts, " ")
			plainGap := innerW - lipgloss.Width(plainLeft) - lipgloss.Width(plainRight)
			if plainGap < 1 {
				plainGap = 1
			}

			if onAgent {
				plain := plainLeft + strings.Repeat(" ", plainGap) + plainRight
				b.WriteString(selectedSt.Width(innerW).Render(plain) + "\n")
				continue
			}

			// styled (non-selected) version
			stColor := m.theme.DisplayStateColor(ds)

			// glyph: ✽ pulses via animFrame
			var styledGlyph string
			if ds == DisplayWorking {
				if m.animFrame%2 == 0 {
					styledGlyph = lipgloss.NewStyle().Foreground(m.theme.Blue).Bold(true).Render("◉")
				} else {
					styledGlyph = lipgloss.NewStyle().Foreground(m.theme.Overlay1).Render("◉")
				}
			} else {
				styledGlyph = lipgloss.NewStyle().Foreground(stColor).Render(plainGlyph)
			}

			agentLbl := lipgloss.NewStyle().Foreground(lipgloss.Color(cfgAgent.Color)).Bold(true).Render(bracketed)

			// intensity varies by urgency
			var nameStyle, msgStyle, metaStyle lipgloss.Style
			switch ds {
			case DisplayNeedsInput:
				nameStyle = lipgloss.NewStyle().Foreground(m.theme.Text).Bold(true)
				msgStyle = lipgloss.NewStyle().Foreground(m.theme.Gold).Italic(true)
				metaStyle = lipgloss.NewStyle().Foreground(m.theme.Subtext0)
			case DisplayWorking:
				nameStyle = lipgloss.NewStyle().Foreground(m.theme.Text)
				msgStyle = lipgloss.NewStyle().Foreground(m.theme.Subtext0)
				metaStyle = lipgloss.NewStyle().Foreground(m.theme.Subtext0)
			case DisplayIdle:
				nameStyle = lipgloss.NewStyle().Foreground(m.theme.Subtext0)
				msgStyle = lipgloss.NewStyle().Foreground(m.theme.Overlay0)
				metaStyle = lipgloss.NewStyle().Foreground(m.theme.Overlay0)
			case DisplayCompleted:
				nameStyle = lipgloss.NewStyle().Foreground(m.theme.Subtext0)
				msgStyle = lipgloss.NewStyle().Foreground(m.theme.Overlay0)
				metaStyle = lipgloss.NewStyle().Foreground(m.theme.Overlay0)
			case DisplayFailed:
				nameStyle = lipgloss.NewStyle().Foreground(m.theme.Subtext1)
				msgStyle = lipgloss.NewStyle().Foreground(m.theme.Subtext0)
				metaStyle = lipgloss.NewStyle().Foreground(m.theme.Subtext0)
			default: // DisplayStopped
				faded := lipgloss.NewStyle().Foreground(m.theme.Overlay0)
				nameStyle, msgStyle, metaStyle = faded, faded, faded
			}

			styledLeft := "  " + styledGlyph + " " + agentLbl + " " + nameStyle.Render(truncName)
			if truncMsg != "" {
				styledLeft += " " + msgStyle.Render(truncMsg)
			}
			var styledRParts []string
			if toolStr != "" {
				styledRParts = append(styledRParts, metaStyle.Render(toolStr))
			}
			styledRParts = append(styledRParts, metaStyle.Render(ageStr))
			styledRight := strings.Join(styledRParts, " ")

			styledGap := innerW - lipgloss.Width(styledLeft) - lipgloss.Width(styledRight)
			if styledGap < 1 {
				styledGap = 1
			}
			b.WriteString(styledLeft + strings.Repeat(" ", styledGap) + styledRight + "\n")
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
		oi := urgencyOrder(ToDisplayState(out[i].State))
		oj := urgencyOrder(ToDisplayState(out[j].State))
		if oi != oj {
			return oi < oj
		}
		if !out[i].StartedAt.Equal(out[j].StartedAt) {
			return out[i].StartedAt.Before(out[j].StartedAt)
		}
		return out[i].ID < out[j].ID
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
