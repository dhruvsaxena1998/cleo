package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
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

	sideW := m.ctx.Config.UI.SidebarWidth
	if sideW > w-40 {
		sideW = w - 40
	}
	if sideW < 10 {
		sideW = 10
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
	sound := lipgloss.NewStyle().Foreground(m.theme.Overlay0).Render("sound on")
	if !m.ctx.Config.Sound.Enabled {
		sound = lipgloss.NewStyle().Foreground(m.theme.Overlay0).Render("muted")
	}
	left := lipgloss.NewStyle().Foreground(m.theme.Mauve).Bold(true).Render("cleo") +
		lipgloss.NewStyle().Foreground(m.theme.Overlay0).Render("  ai agents")
	memMB := float64(m.heapAlloc) / (1024 * 1024)
	right := fmt.Sprintf("%s  %s  %s  %s  %s",
		m.theme.Pill(fmt.Sprintf("%d projects", len(m.projects)), m.theme.Subtext0),
		m.theme.Pill(fmt.Sprintf("%d live", stats.live), m.theme.Green),
		m.theme.Pill(fmt.Sprintf("%d waiting", stats.waiting), m.theme.Peach),
		m.theme.Pill(fmt.Sprintf("%.1f MB", memMB), m.theme.Overlay0),
		sound,
	)
	space := width - lipgloss.Width(left) - lipgloss.Width(right) - 2
	if space < 1 {
		space = 1
	}
	return lipgloss.NewStyle().Background(m.theme.Mantle).Foreground(m.theme.Text).Padding(0, 1).
		Width(width).Render(left + strings.Repeat(" ", space) + right)
}

// ── Footer ────────────────────────────────────────────────────────────────────

func (m Model) renderFooter(width int) string {
	faint := lipgloss.NewStyle().Foreground(m.theme.Overlay0)
	sep := faint.Render("  ·  ")

	var hints []string
	if m.status != "" && m.mode != ModeFilter {
		hints = []string{
			lipgloss.NewStyle().Foreground(m.theme.Red).Bold(true).Render(m.status),
			m.theme.KeyHint("esc", "clear"),
			m.theme.KeyHint("q", "quit"),
		}
		line := "  " + strings.Join(hints, sep)
		return lipgloss.NewStyle().Background(m.theme.Base).Width(width).Render(truncateWidth(line, width))
	}
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
			if sess.State.IsFinished() {
				hints = []string{
					faint.Render(m.statusOr(fmt.Sprintf("%s is %s", sess.ID, sess.State))),
					m.theme.KeyHint("K", "remove"),
					m.theme.KeyHint("P", "prune project"),
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
					m.theme.KeyHint("m", "send"),
					m.theme.KeyHint("q", "quit"),
				}
			}
		} else {
			pid, _ := m.projectAtCursor()
			var hasFinished bool
			for _, s := range m.sessions {
				if s.ProjectID == pid && s.State.IsFinished() {
					hasFinished = true
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
				m.theme.KeyHint("m", "send"),
				m.theme.KeyHint("q", "quit"),
			}
			if hasFinished {
				hints = append(hints, m.theme.KeyHint("P", "prune"))
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

// ── Pruning banner ────────────────────────────────────────────────────────────

func (m Model) retentionBanner(width int) string {
	threshold := m.ctx.Config.Pruning.HintThreshold
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
		leftPart := ansi.Truncate(baseLine, left, "")
		rightPart := ansi.TruncateLeft(baseLine, left+overlayW, "")
		baseLines[idx] = leftPart + ol + rightPart
	}
	return strings.Join(baseLines, "\n")
}
