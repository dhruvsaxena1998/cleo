package tui

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		return m, nil
	case stateLoadedMsg:
		m.projects = msg.projects
		m.sessions = msg.sessions
		return m, nil
	case tickStateMsg:
		return m, tea.Batch(loadStateCmd(m.ctx), tickStateCmd())
	case tea.KeyMsg:
		return m.handleKey(msg)
	case paneCapturedMsg:
		if m.paneCache == nil {
			m.paneCache = map[string]string{}
		}
		m.paneCache[msg.sid] = msg.content
		// Re-tick capture for the currently-selected session.
		if m.selected != "" && m.selected == msg.sid {
			return m, tea.Tick(m.ctx.Config.UI.PanePreviewInterval, func(time.Time) tea.Msg {
				return capturePaneTickMsg{sid: msg.sid}
			})
		}
		return m, nil
	case capturePaneTickMsg:
		if m.selected == msg.sid {
			return m, capturePaneCmd(m.ctx, msg.sid, m.ctx.Config.UI.PanePreviewLines)
		}
		return m, nil
	}
	return m, nil
}
