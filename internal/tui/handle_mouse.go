package tui

import (
	tea "github.com/charmbracelet/bubbletea"
	zone "github.com/lrstanley/bubblezone"
)

// handleMouse routes mouse events. Wheel scrolls the sidebar tree selection;
// left-click hit-tests the clickable row zones marked in renderTreeContent and
// either selects, attaches, or toggles expansion. All cursor/attach work reuses
// the same helpers as the keyboard path (handle_key.go), so behaviour stays in
// lockstep with key navigation.
func (m Model) handleMouse(msg tea.MouseMsg) (Model, tea.Cmd) {
	// While a popup is open, forward to it so bubbles' textinput gets its
	// built-in click-to-position for free. Explicit clickable popup controls
	// are out of scope.
	if m.mode == ModePopup && m.popup != nil {
		var cmd tea.Cmd
		m.popup, cmd = m.popup.Update(msg)
		return m, cmd
	}

	// Wheel events are position-independent: the tree is the only scrollable
	// list, so map them straight onto cursor movement (which also refreshes the
	// preview via autoCaptureCmd).
	switch msg.Button {
	case tea.MouseButtonWheelUp:
		return m.cursorUp()
	case tea.MouseButtonWheelDown:
		return m.cursorDown()
	}

	if msg.Action != tea.MouseActionPress || msg.Button != tea.MouseButtonLeft {
		return m, nil
	}
	return m.handleTreeClick(msg)
}

// handleTreeClick maps a left-click to a tree row. A project header click
// selects that project and toggles its expansion; a session-row click selects
// it, and clicking the already-selected session attaches to it ("click to
// select, click again to attach" — no double-click timer needed).
func (m Model) handleTreeClick(msg tea.MouseMsg) (Model, tea.Cmd) {
	projs := m.visibleProjects()
	for pi, p := range projs {
		if zone.Get(projZoneID(pi)).InBounds(msg) {
			m.clearStatus()
			m.cursor = cursor{projectIdx: pi, agentIdx: projectRow}
			m.expanded[p.ID] = !m.expanded[p.ID]
			return m, m.autoCaptureCmd()
		}
		if !m.expanded[p.ID] {
			continue
		}
		for ai := range m.sessionsFor(p.ID) {
			if !zone.Get(sessZoneID(pi, ai)).InBounds(msg) {
				continue
			}
			alreadySelected := m.cursor.onAgent(pi, ai)
			m.clearStatus()
			m.cursor = cursor{projectIdx: pi, agentIdx: ai}
			if alreadySelected {
				return m.attachSelectedAgent()
			}
			return m, m.autoCaptureCmd()
		}
	}
	return m, nil
}
