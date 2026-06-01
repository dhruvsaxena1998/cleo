package tui

import (
	tea "github.com/charmbracelet/bubbletea"
)

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		return m, nil
	case stateLoadedMsg:
		firstLoad := !m.firstStateLoaded
		m.firstStateLoaded = true
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
		// On the very first state load, fire one immediate capture so the
		// preview panel renders within ~tmux-ls latency instead of waiting
		// for the first previewTickCmd interval (~1.5s of "loading…").
		if firstLoad {
			if cmd := m.autoCaptureCmd(); cmd != nil {
				m.paneCaptureInFlight = true
				return m, cmd
			}
		}
		return m, nil
	case tickStateMsg:
		m.heapAlloc = readHeapAlloc()
		return m, tea.Batch(loadStateCmd(m.ctx), tickStateCmd())
	case agentMemTickMsg:
		return m, tea.Batch(agentMemCmd(m.ctx), agentMemTickCmd())
	case agentMemMsg:
		m.agentMemAlloc = msg.bytes
		return m, nil
	case statusExpiredMsg:
		if msg.id == m.statusTimerID {
			m.status = ""
		}
		return m, nil
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
		switch msg.Kind {
		case confirmKindPrune:
			return m.performPrune(msg.Target)
		case confirmKindRemoveProject:
			return m.performRemoveProject(msg.Target)
		default:
			return m.performKill(msg.Target)
		}
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
	case SendSubmitted:
		return m.performSend(msg)
	case SendCancelled:
		m.status = ""
		m.mode = ModeNormal
		m.popup = nil
		return m, nil
	case HelpClosed:
		m.mode = ModeNormal
		m.popup = nil
		return m, nil
	case paneCapturedMsg:
		if m.paneCache == nil {
			m.paneCache = map[string]string{}
		}
		m.paneCaptureInFlight = false
		// Diff before render: skip cache update when content is unchanged so
		// Bubble Tea's own output dedup avoids a repaint of the preview panel.
		if m.paneCache[msg.sid] != msg.content {
			m.paneCache[msg.sid] = msg.content
		}
		return m, nil
	case previewTickMsg:
		next := previewTickCmd(m.ctx.Config.UI.PanePreview.Interval)
		if !m.ctx.Config.UI.PanePreview.Enabled {
			return m, next
		}
		sess, ok := m.selectedSession()
		if !ok || sess.State.IsFinished() || m.paneCaptureInFlight {
			return m, next
		}
		m.paneCaptureInFlight = true
		return m, tea.Batch(next, capturePaneCmd(m.ctx, sess.ID, m.ctx.Config.UI.PanePreview.Lines))
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
