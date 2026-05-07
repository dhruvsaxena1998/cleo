package tui

import tea "github.com/charmbracelet/bubbletea"

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
	}
	return m, nil
}
