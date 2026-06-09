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

		var cmds []tea.Cmd
		// Start the spinner loop when work appears and it is not already running.
		// The animTicking guard keeps the 750ms state tick from arming a second,
		// overlapping ticker on every reload.
		if m.hasWorkingSession() && !m.animTicking {
			m.animTicking = true
			cmds = append(cmds, animTickCmd())
		}
		// On the very first state load, fire one immediate capture so the
		// preview panel renders within ~tmux-ls latency instead of waiting
		// for the first previewTickCmd interval (~1.5s of "loading…").
		if firstLoad {
			if cmd := m.autoCaptureCmd(); cmd != nil {
				m.paneCaptureInFlight = true
				cmds = append(cmds, cmd)
			}
		}
		return m, tea.Batch(cmds...)
	case animTickMsg:
		// Re-arm only while work remains; otherwise let the loop die and clear
		// the guard so the next stateLoadedMsg can restart it.
		if !m.hasWorkingSession() {
			m.animTicking = false
			return m, nil
		}
		m.animFrame++
		return m, animTickCmd()
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
	case editorFinishedMsg:
		if msg.err != nil {
			return m, m.setStatus("editor failed: " + msg.err.Error())
		}
		return m, nil
	case tea.KeyMsg:
		return m.handleKey(msg)
	case tea.MouseMsg:
		return m.handleMouse(msg)
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
	case FinderSubmitted:
		m.mode = ModeNormal
		m.popup = nil
		return m.attachToSession(msg.SessionID)
	case FinderCancelled:
		m.mode = ModeNormal
		m.popup = nil
		return m, nil
	case SettingsChanged:
		// Live preview only — apply to the in-memory config (and re-resolve the
		// theme) so the dashboard recolors/resizes; nothing is written yet. The
		// returned command re-syncs the terminal background when the theme moved.
		m.ctx.Config = msg.Config
		cmd := m.applyTheme(msg.Config.UI.Theme)
		return m, cmd
	case SettingsSaved:
		return m.performSettingsSave(msg)
	case HelpClosed:
		m.mode = ModeNormal
		m.popup = nil
		return m, nil
	case WarningsClosed:
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
	m.cursor = m.cursor.clamp(m.treeShape())
	return m
}
