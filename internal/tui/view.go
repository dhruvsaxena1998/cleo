package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
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

	parts := []string{m.renderTopbar(w), body, m.renderFooter(w)}
	return strings.Join(parts, "\n")
}

// ── Topbar ────────────────────────────────────────────────────────────────────

func (m Model) renderTopbar(width int) string {
	stats := m.sessionStats()
	sound := styleFaint.Render("sound on")
	if !m.ctx.Config.Sound.Enabled {
		sound = styleFaint.Render("muted")
	}
	left := styleApp.Render("cleo") + styleFaint.Render("  ai agents")
	right := fmt.Sprintf("%s  %s  %s  %s",
		pill(fmt.Sprintf("%d projects", len(m.projects)), clrSubtext),
		pill(fmt.Sprintf("%d live", stats.live), clrGreen),
		pill(fmt.Sprintf("%d waiting", stats.waiting), clrAmber),
		sound,
	)
	space := width - lipgloss.Width(left) - lipgloss.Width(right) - 2
	if space < 1 {
		space = 1
	}
	return styleTopbar.Width(width).Render(left + strings.Repeat(" ", space) + right)
}

// ── Footer ────────────────────────────────────────────────────────────────────

func (m Model) renderFooter(width int) string {
	sep := styleFaint.Render("  ·  ")

	var hints []string
	switch {
	case m.mode == ModeFilter:
		hints = []string{
			keyHint("enter", "apply"),
			keyHint("esc", "clear"),
			styleFaint.Render("type to filter projects and sessions"),
		}
	default:
		sess, hasSess := m.sessionAtCursor()
		if hasSess {
			// Primary action first
			if sess.State.IsFinished() {
				hints = []string{
					styleFaint.Render(m.statusOr(fmt.Sprintf("%s is %s", sess.ID, sess.State))),
					keyHint("K", "remove"),
					keyHint("n", "new sibling"),
					keyHint("/", "filter"),
					keyHint("q", "quit"),
				}
			} else {
				hints = []string{
					keyHint("↵ ", "attach"),
					keyHint("v", "view"),
					keyHint("r", "rename"),
					keyHint("K", "kill"),
					keyHint("n", "new sibling"),
					keyHint("space", "collapse"),
					keyHint("/", "filter"),
					keyHint("m", "mute"),
					keyHint("q", "quit"),
				}
			}
		} else {
			hints = []string{
				keyHint("n", "new session"),
				keyHint("space", "expand"),
				keyHint("j/k", "move"),
				keyHint("↵ ", "attach"),
				keyHint("/", "filter"),
				keyHint("m", "mute"),
				keyHint("q", "quit"),
			}
		}
	}

	line := "  " + strings.Join(hints, sep)
	return truncateWidth(line, width)
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
				Foreground(clrGold).
				Background(clrRaised).
				Width(width).
				Render(truncateWidth(msg, width-2))
		}
	}
	return ""
}

// ── Session stats ─────────────────────────────────────────────────────────────

type sessionSummary struct {
	live, waiting, finished, errored int
}

func (m Model) sessionStats() sessionSummary {
	var s sessionSummary
	for _, sess := range m.sessions {
		switch sess.State {
		case "running", "spawning", "idle":
			s.live++
		case "waiting_for_input":
			s.waiting++
		case "error":
			s.errored++
			s.finished++
		case "completed", "dead":
			s.finished++
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
		bw := lipgloss.Width(baseLine)
		// Clear the region behind the overlay
		padding := ""
		if left > 0 && left <= bw {
			padding = strings.Repeat(" ", left)
			_ = padding
		}
		// Replace the section
		baseLines[idx] = strings.Repeat(" ", left) + ol
		_ = bw
	}
	return strings.Join(baseLines, "\n")
}
