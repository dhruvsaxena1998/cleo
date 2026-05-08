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
		// Auto-expand projects that have sessions on first discovery.
		for _, p := range msg.projects {
			if _, known := m.expanded[p.ID]; !known {
				for _, s := range msg.sessions {
					if s.ProjectID == p.ID {
						m.expanded[p.ID] = true
						break
					}
				}
			}
		}
		m = m.clampCursor()
		return m, nil
	case tickStateMsg:
		return m, tea.Batch(loadStateCmd(m.ctx), tickStateCmd())
	case tea.KeyMsg:
		return m.handleKey(msg)
	case SpawnSubmitted:
		return m.performSpawn(msg)
	case SpawnCancelled:
		m.status = ""
		m.mode = ModeNormal
		m.popup = nil
		return m, nil
	case ConfirmYes:
		return m.performKill(msg.Target)
	case ConfirmNo:
		m.status = ""
		m.mode = ModeNormal
		m.popup = nil
		return m, nil
	case RenameSubmitted:
		return m.performRename(msg)
	case RenameCancelled:
		m.status = ""
		m.mode = ModeNormal
		m.popup = nil
		return m, nil
	case paneCapturedMsg:
		if m.paneCache == nil {
			m.paneCache = map[string]string{}
		}
		m.paneCache[msg.sid] = msg.content
		// Schedule re-capture if the session that just landed is still visible.
		if visible, ok := m.selectedSession(); ok && visible.ID == msg.sid {
			if visible.State.IsFinished() {
				return m, nil
			}
			return m, tea.Tick(m.ctx.Config.UI.PanePreviewInterval, func(time.Time) tea.Msg {
				return capturePaneTickMsg{sid: msg.sid}
			})
		}
		return m, nil
	case capturePaneTickMsg:
		// Only re-capture if this session is still the one on screen.
		if visible, ok := m.selectedSession(); ok && visible.ID == msg.sid {
			if visible.State.IsFinished() {
				return m, nil
			}
			return m, capturePaneCmd(m.ctx, msg.sid, m.ctx.Config.UI.PanePreviewLines)
		}
		return m, nil
	}
	return m, nil
}

func (m Model) clampCursor() Model {
	projects := m.visibleProjectIDs()
	if len(projects) == 0 {
		m.cursor.projectIdx = 0
		m.cursor.agentIdx = -1
		return m
	}
	if m.cursor.projectIdx < 0 {
		m.cursor.projectIdx = 0
	}
	if m.cursor.projectIdx >= len(projects) {
		m.cursor.projectIdx = len(projects) - 1
	}
	pid := projects[m.cursor.projectIdx]
	if !m.expanded[pid] {
		m.cursor.agentIdx = -1
		return m
	}
	ss := m.sessionsFor(pid)
	if m.cursor.agentIdx >= len(ss) {
		m.cursor.agentIdx = len(ss) - 1
	}
	if m.cursor.agentIdx < -1 {
		m.cursor.agentIdx = -1
	}
	return m
}
