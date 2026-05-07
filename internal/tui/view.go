package tui

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"
)

func (m Model) View() string {
	banner := m.retentionBanner()
	out := renderFrame(m)
	if banner != "" {
		out = banner + "\n" + out
	}
	if m.mode == ModePopup && m.popup != nil {
		out += "\n\n" + m.popup.View()
	}
	return out
}

func renderFrame(m Model) string {
	side := m.renderSidebar(m.ctx.Config.UI.SidebarWidth)
	main := m.renderMain(m.width - m.ctx.Config.UI.SidebarWidth - 4)
	frame := lipgloss.JoinHorizontal(lipgloss.Top,
		stylePanel.Width(m.ctx.Config.UI.SidebarWidth).Render(side),
		stylePanel.Width(m.width-m.ctx.Config.UI.SidebarWidth-4).Render(main),
	)
	return frame + "\n" + m.renderFooter()
}

func (m Model) renderFooter() string {
	return styleDimmed.Render("n new  v view  ↵ attach  K kill  / filter  m mute  ? help  q quit")
}

func (m Model) retentionBanner() string {
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
			return styleDimmed.Render(fmt.Sprintf("💡 '%s' has %d finished sessions — run `cleo prune %s` to clean up", pid, n, pid))
		}
	}
	return ""
}
