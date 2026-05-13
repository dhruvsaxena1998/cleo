package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/x/ansi"
	"github.com/charmbracelet/lipgloss"

	"github.com/dhruvsaxena1998/cleo/internal/state"
)

func (m Model) View() string {
	out := renderFrame(m)
	if m.mode == ModePopup && m.popup != nil {
		out = m.renderOverlay(out, m.popup.View())
	}
	return out
}

func renderFrame(m Model) string {
	w := m.width
	h := m.height
	if w <= 0 {
		w = 120
	}
	if h <= 0 {
		h = 40
	}

	topH := 1
	footH := 1
	bodyH := h - topH - footH
	if bodyH < 8 {
		bodyH = 8
	}

	sideW := w * 36 / 100
	if sideW < 32 {
		sideW = 32
	}
	mainW := w - sideW
	if mainW < 40 {
		mainW = 40
	}

	left := m.renderLeftColumn(sideW, bodyH)
	right := m.renderRightColumn(mainW, bodyH)
	body := lipgloss.JoinHorizontal(lipgloss.Top, left, right)

	// Stamp the theme's base background on every line so that any transparent
	// characters (spaces between ANSI-styled spans) show the theme colour
	// instead of the terminal default.
	baseSt := lipgloss.NewStyle().Background(m.theme.Base).Width(w)
	rows := strings.Split(strings.Join([]string{m.renderTopbar(w), body, m.renderFooter(w)}, "\n"), "\n")
	for i, row := range rows {
		rows[i] = baseSt.Render(row)
	}
	return strings.Join(rows, "\n")
}

// ── Topbar ────────────────────────────────────────────────────────────────────

func (m Model) renderTopbar(width int) string {
	stats := m.sessionStats()

	accent := lipgloss.NewStyle().Foreground(m.theme.Mauve).Bold(true)
	faint := lipgloss.NewStyle().Foreground(m.theme.Overlay0)

	left := accent.Render("◈ cleo") +
		faint.Render("  ·  ") +
		faint.Render(fmt.Sprintf("%d projects", len(m.projects))) +
		faint.Render(" · ") +
		faint.Render(fmt.Sprintf("%d sessions", stats.total))

	var rightParts []string

	if stats.needsInput > 0 {
		var badge string
		if m.animFrame%2 == 0 {
			badge = lipgloss.NewStyle().
				Background(m.theme.Gold).
				Foreground(lipgloss.Color("#11111b")).
				Bold(true).
				Padding(0, 1).
				Render(fmt.Sprintf("⚠ %d needs input", stats.needsInput))
		} else {
			badge = lipgloss.NewStyle().
				Foreground(m.theme.Gold).
				Bold(true).
				Render(fmt.Sprintf("⚠ %d needs input", stats.needsInput))
		}
		rightParts = append(rightParts, badge)
	}

	if stats.working > 0 {
		badge := lipgloss.NewStyle().
			Foreground(m.theme.Blue).
			Render(fmt.Sprintf("✽ %d working", stats.working))
		rightParts = append(rightParts, badge)
	}

	memMB := float64(m.heapAlloc) / (1024 * 1024)
	rightParts = append(rightParts, faint.Render(fmt.Sprintf("%.1fMB", memMB)))

	soundLabel := "♪ on"
	if m.ctx.Config.Sound.Enabled != nil && !*m.ctx.Config.Sound.Enabled {
		soundLabel = "♪ off"
	}
	rightParts = append(rightParts, faint.Render(soundLabel))

	sep := faint.Render("  ·  ")
	right := strings.Join(rightParts, sep)

	space := width - lipgloss.Width(left) - lipgloss.Width(right) - 2
	if space < 1 {
		space = 1
	}
	return lipgloss.NewStyle().Background(m.theme.Mantle).Foreground(m.theme.Text).
		Padding(0, 1).Width(width).
		Render(left + strings.Repeat(" ", space) + right)
}

// ── Footer ────────────────────────────────────────────────────────────────────

func (m Model) renderFooter(width int) string {
	faint := lipgloss.NewStyle().Foreground(m.theme.Overlay0)
	sep := faint.Render("  ·  ")

	var hints []string
	switch {
	case m.mode == ModeFilter:
		hints = []string{
			m.theme.KeyHint("enter", "apply"),
			m.theme.KeyHint("esc", "clear"),
			faint.Render("type to filter projects and sessions"),
		}
	default:
		sess, hasSess := m.sessionAtCursor()
		if hasSess {
			if sess.State == state.Dead {
				hints = []string{
					faint.Render(m.statusOr(fmt.Sprintf("%s is stopped", sess.ID))),
					m.theme.KeyHint("K", "remove"),
					m.theme.KeyHint("P", "prune stopped"),
					m.theme.KeyHint("n", "new sibling"),
					m.theme.KeyHint("/", "filter"),
					m.theme.KeyHint("q", "quit"),
				}
			} else {
				hints = []string{
					m.theme.KeyHint("↵ ", "attach"),
					m.theme.KeyHint("v", "view"),
					m.theme.KeyHint("r", "rename"),
					m.theme.KeyHint("K", "kill"),
					m.theme.KeyHint("n", "new sibling"),
					m.theme.KeyHint("space", "collapse"),
					m.theme.KeyHint("/", "filter"),
					m.theme.KeyHint("m", "mute"),
					m.theme.KeyHint("q", "quit"),
				}
			}
		} else {
			pid, _ := m.projectAtCursor()
			var hasStopped bool
			for _, s := range m.sessions {
				if s.ProjectID == pid && s.State == state.Dead {
					hasStopped = true
					break
				}
			}
			hints = []string{
				m.theme.KeyHint("n", "new session"),
				m.theme.KeyHint("space", "expand"),
				m.theme.KeyHint("j/k", "move"),
				m.theme.KeyHint("↵ ", "attach"),
				m.theme.KeyHint("D", "remove project"),
				m.theme.KeyHint("/", "filter"),
				m.theme.KeyHint("m", "mute"),
				m.theme.KeyHint("q", "quit"),
			}
			if hasStopped {
				hints = append(hints, m.theme.KeyHint("P", "prune stopped"))
			}
		}
	}

	line := "  " + strings.Join(hints, sep)
	return lipgloss.NewStyle().Background(m.theme.Base).Width(width).Render(truncateWidth(line, width))
}

func (m Model) statusOr(fallback string) string {
	if m.status != "" {
		return m.status
	}
	return fallback
}

// ── Retention banner ──────────────────────────────────────────────────────────

func (m Model) retentionBanner(width int) string {
	threshold := m.ctx.Config.Retention.HintThreshold
	if threshold <= 0 {
		return ""
	}
	counts := map[string]int{}
	for _, s := range m.sessions {
		if s.State.IsFinished() {
			counts[s.ProjectID]++
		}
	}
	for pid, n := range counts {
		if n > threshold {
			msg := fmt.Sprintf("  hint  %s has %d finished sessions  run: cleo prune %s", pid, n, pid)
			return lipgloss.NewStyle().
				Foreground(m.theme.Gold).
				Background(m.theme.Surf0).
				Width(width).
				Render(truncateWidth(msg, width-2))
		}
	}
	return ""
}

// ── Session stats ─────────────────────────────────────────────────────────────

type sessionSummary struct {
	total, needsInput, working int
}

func (m Model) sessionStats() sessionSummary {
	var s sessionSummary
	s.total = len(m.sessions)
	for _, sess := range m.sessions {
		switch ToDisplayState(sess.State) {
		case DisplayNeedsInput:
			s.needsInput++
		case DisplayWorking:
			s.working++
		}
	}
	return s
}

// ── Overlay (popup) ───────────────────────────────────────────────────────────

func (m Model) renderOverlay(base, overlay string) string {
	width := m.width
	if width <= 0 {
		width = lipgloss.Width(overlay) + 8
	}
	overlayW := lipgloss.Width(overlay)
	left := (width - overlayW) / 2
	if left < 0 {
		left = 0
	}
	// Place overlay vertically in the middle
	baseLines := strings.Split(base, "\n")
	overlayLines := strings.Split(overlay, "\n")
	top := (len(baseLines) - len(overlayLines)) / 2
	if top < 0 {
		top = 0
	}
	for i, ol := range overlayLines {
		idx := top + i
		if idx >= len(baseLines) {
			break
		}
		baseLine := baseLines[idx]
		leftPart := ansi.Truncate(baseLine, left, "")
		rightPart := ansi.TruncateLeft(baseLine, left+overlayW, "")
		baseLines[idx] = leftPart + ol + rightPart
	}
	return strings.Join(baseLines, "\n")
}
