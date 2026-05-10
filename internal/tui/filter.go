package tui

import tea "github.com/charmbracelet/bubbletea"

func (m Model) handleFilterKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Esc is intercepted in handleKey (spec §2.2 hierarchy); only Enter,
	// backspace, and rune input land here.
	switch msg.Type {
	case tea.KeyEnter:
		m.mode = ModeNormal
		return m.clampCursor(), nil
	case tea.KeyBackspace:
		if len(m.filter) > 0 {
			m.filter = m.filter[:len(m.filter)-1]
		}
		return m.clampCursor(), nil
	case tea.KeyRunes:
		m.filter += string(msg.Runes)
		return m.clampCursor(), nil
	}
	return m, nil
}
