package tui

import tea "github.com/charmbracelet/bubbletea"

func (m Model) handleFilterKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEsc:
		m.filter = ""
		m.mode = ModeNormal
		return m, nil
	case tea.KeyEnter:
		m.mode = ModeNormal
		return m, nil
	case tea.KeyBackspace:
		if len(m.filter) > 0 {
			m.filter = m.filter[:len(m.filter)-1]
		}
		return m, nil
	case tea.KeyRunes:
		m.filter += string(msg.Runes)
		return m, nil
	}
	return m, nil
}
