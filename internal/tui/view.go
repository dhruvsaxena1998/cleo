package tui

import "github.com/charmbracelet/lipgloss"

func (m Model) View() string {
	out := renderFrame(m)
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
	return styleDimmed.Render("n new  v view  ↵ attach  k kill  / filter  m mute  ? help  q quit")
}
