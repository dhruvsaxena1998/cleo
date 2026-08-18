package tui

import (
	"fmt"
	"os"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/dhruvsaxena1998/cleo/internal/config"
	"github.com/dhruvsaxena1998/cleo/internal/sessionlifecycle"
	"github.com/dhruvsaxena1998/cleo/internal/state"
)

func (m Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Reserved ctrl+c hatch: always quit, in every mode, regardless of config.
	// Intercepted before popup forwarding so the user can never be locked out.
	if msg.Type == tea.KeyCtrlC {
		return m, tea.Quit
	}
	// Explicit Esc hierarchy: popup -> status.
	// Intercepted at the top level so each layer behaves predictably no
	// matter which mode forwarded the keypress.
	if msg.Type == tea.KeyEsc {
		if m.mode == ModePopup && m.popup != nil {
			// The settings popup previews edits live against ctx.Config; esc is
			// cancel, so restore the pre-open snapshot (and theme) before closing.
			var cmd tea.Cmd
			if _, ok := m.popup.(SettingsPopup); ok {
				m.ctx.Config = m.settingsBackup
				cmd = m.applyTheme(m.settingsBackup.UI.Theme)
			}
			m.popup = nil
			m.mode = ModeNormal
			m.status = ""
			return m, cmd
		}
		m.status = ""
		return m, nil
	}
	if m.mode == ModePopup && m.popup != nil {
		var cmd tea.Cmd
		m.popup, cmd = m.popup.Update(msg)
		return m, cmd
	}
	// Reserved enter hatch: in the main view enter always attaches the
	// selection, even if the attach action was rebound away from enter.
	if msg.Type == tea.KeyEnter {
		return m.attachSelectedAgent()
	}
	km := m.ctx.Config.Keymap
	switch {
	case key.Matches(msg, km.Quit):
		return m, tea.Quit
	case key.Matches(msg, km.Filter):
		return m.openFinderPopup()
	case key.Matches(msg, km.New):
		return m.openSpawnPopup()
	case key.Matches(msg, km.View):
		return m.viewSelectedAgent()
	case key.Matches(msg, km.Enter):
		return m.attachSelectedAgent()
	case key.Matches(msg, km.Editor):
		return m.openSelectedProjectInEditor()
	case key.Matches(msg, km.Kill):
		return m.confirmKill()
	case key.Matches(msg, km.Prune):
		return m.confirmPrune()
	case key.Matches(msg, km.Remove):
		return m.confirmRemoveProject()
	case key.Matches(msg, km.Rename):
		return m.openRenamePopup()
	case key.Matches(msg, km.Send):
		return m.openSendPopup()
	case key.Matches(msg, km.Mute):
		return m.toggleMute()
	case key.Matches(msg, km.Settings):
		return m.openSettingsPopup()
	case key.Matches(msg, km.Help):
		return m.openHelpPopup()
	case key.Matches(msg, km.Up):
		return m.cursorUp()
	case key.Matches(msg, km.Down):
		return m.cursorDown()
	case key.Matches(msg, km.Space):
		return m.toggleExpand()
	}
	return m, nil
}

// --- helpers ---

func (m Model) projectAtCursor() (string, bool) {
	projs := m.visibleProjectIDs()
	if m.cursor.projectIdx < 0 || m.cursor.projectIdx >= len(projs) {
		return "", false
	}
	return projs[m.cursor.projectIdx], true
}

func (m Model) visibleProjectIDs() []string {
	projects := m.visibleProjects()
	out := make([]string, 0, len(projects))
	for _, p := range projects {
		out = append(out, p.ID)
	}
	return out
}

func (m Model) sessionAtCursor() (state.Session, bool) {
	pid, ok := m.projectAtCursor()
	if !ok {
		return state.Session{}, false
	}
	if !m.expanded[pid] || m.cursor.agentIdx == projectRow {
		return state.Session{}, false
	}
	ss := m.sessionsFor(pid)
	if m.cursor.agentIdx >= len(ss) {
		return state.Session{}, false
	}
	return ss[m.cursor.agentIdx], true
}

func (m Model) toggleExpand() (Model, tea.Cmd) {
	m.status = ""
	pid, ok := m.projectAtCursor()
	if !ok {
		return m, nil
	}
	m.expanded[pid] = !m.expanded[pid]
	return m, nil
}

func (m Model) cursorUp() (Model, tea.Cmd) {
	m.clearStatus()
	m.cursor = m.cursor.up(m.treeShape())
	return m, m.autoCaptureCmd()
}

func (m Model) cursorDown() (Model, tea.Cmd) {
	m.clearStatus()
	m.cursor = m.cursor.down(m.treeShape())
	return m, m.autoCaptureCmd()
}

// autoCaptureCmd fires a fresh pane capture for the cursor session so the
// preview stays current when the user navigates. Skips when previews are
// disabled, no session is selected, or the session is finished.
func (m Model) autoCaptureCmd() tea.Cmd {
	if !m.ctx.Config.UI.PanePreview.Enabled {
		return nil
	}
	sess, ok := m.sessionAtCursor()
	if !ok {
		return nil
	}
	if sess.State.IsFinished() {
		return nil
	}
	return capturePaneCmd(m.ctx, sess.ID, m.ctx.Config.UI.PanePreview.Lines)
}

func (m Model) openSpawnPopup() (Model, tea.Cmd) {
	m.status = ""
	pid, ok := m.projectAtCursor()

	var defaultPID string
	if ok {
		defaultPID = pid
	}

	agents := []string{}
	for k := range m.ctx.Config.Agents {
		agents = append(agents, k)
	}

	cwd, _ := os.Getwd()

	m.popup = NewSpawnPopup(defaultPID, m.projects, cwd, agents, m.ctx.Config.DefaultAgent, m.theme)
	m.mode = ModePopup
	return m, m.popup.Init()
}

func (m Model) viewSelectedAgent() (Model, tea.Cmd) {
	sess, ok := m.sessionAtCursor()
	if !ok {
		return m, nil
	}
	if sess.State.IsFinished() {
		return m, m.setStatus(fmt.Sprintf("%s is %s; terminal is no longer attached", sess.ID, sess.State))
	}
	m.selected = sess.ID
	return m, capturePaneCmd(m.ctx, sess.ID, m.ctx.Config.UI.PanePreview.Lines)
}

func (m Model) attachToSession(sessionID string) (Model, tea.Cmd) {
	lifecycle := m.ctx.NewLifecycle()

	plan, err := lifecycle.Attach(sessionID)
	if err != nil {
		return m, m.setStatus(fmt.Sprintf("attach failed: %v", err))
	}

	switch plan.Action {
	case sessionlifecycle.AttachBlocked:
		return m, m.setStatus(fmt.Sprintf("%s is %s; press K to remove it", sessionID, plan.Session.State))
	case sessionlifecycle.AttachMarkedDead:
		return m, tea.Batch(m.setStatus(fmt.Sprintf("%s is no longer running; marked dead", sessionID)), loadStateCmd(m.ctx))
	}

	// AttachReady or AttachRevived — proceed with attaching. Done clears focus
	// once the user detaches and Bubble Tea resumes; attachExitedMsg lets Update
	// re-arm mouse tracking, which ExecProcess leaves disabled (see resumeMouseCmd).
	done := plan.Done
	return m, tea.ExecProcess(plan.Cmd, func(err error) tea.Msg {
		done()
		return attachExitedMsg{}
	})
}

// attachExitedMsg is delivered when a tmux attach run via ExecProcess returns
// (the user detached). It exists so Update can re-enable mouse tracking on resume.
type attachExitedMsg struct{}

func (m Model) attachSelectedAgent() (Model, tea.Cmd) {
	sess, ok := m.sessionAtCursor()
	if !ok {
		return m, nil
	}
	return m.attachToSession(sess.ID)
}

func (m Model) confirmKill() (Model, tea.Cmd) {
	sess, ok := m.sessionAtCursor()
	if !ok {
		return m, nil
	}
	// Status clear comes after the early return: pressing 'K' on an empty
	// row is a no-op and shouldn't clear an existing status message.
	m.status = ""
	m.popup = NewConfirmPopup("Confirm Kill", "confirm kill", fmt.Sprintf("kill %q?", sess.ID), sess.ID, confirmKindKill, m.theme)
	m.mode = ModePopup
	return m, m.popup.Init()
}

func (m Model) openRenamePopup() (Model, tea.Cmd) {
	sess, ok := m.sessionAtCursor()
	if !ok {
		return m, nil
	}
	if sess.State.IsFinished() {
		// Replace status with the finished-session warning rather than clearing
		// first; otherwise a reader sees status= "" then immediately reassigned.
		return m, m.setStatus(fmt.Sprintf("%s is %s; finished sessions cannot be renamed", sess.ID, sess.State))
	}
	// Clear stale status only on the success path (popup actually opens).
	m.status = ""
	m.popup = NewRenamePopup(sess.ID, sess.Name, m.theme)
	m.mode = ModePopup
	return m, m.popup.Init()
}

func (m Model) openFinderPopup() (Model, tea.Cmd) {
	m.status = ""
	m.popup = NewFinderPopup(m.ctx, m.theme, m.sessions)
	m.mode = ModePopup
	return m, m.popup.Init()
}

func (m Model) openHelpPopup() (Model, tea.Cmd) {
	m.status = ""
	m.popup = NewHelpPopup(m.theme, m.ctx.Config.Keymap, m.ctx.Config.Tmux.DetachKey)
	m.mode = ModePopup
	return m, m.popup.Init()
}

func (m Model) openSettingsPopup() (Model, tea.Cmd) {
	m.status = ""
	// Snapshot the current config so an esc cancel can revert the live preview.
	m.settingsBackup = m.ctx.Config
	agents := make([]string, 0, len(m.ctx.Config.Agents))
	for name := range m.ctx.Config.Agents {
		agents = append(agents, name)
	}
	m.popup = NewSettingsPopup(m.ctx.Config, m.theme, agents, m.height)
	m.mode = ModePopup
	return m, m.popup.Init()
}

func (m Model) openSendPopup() (Model, tea.Cmd) {
	sess, ok := m.sessionAtCursor()
	if !ok {
		return m, m.setStatus("select a session with j/k first")
	}
	if sess.State.IsFinished() {
		return m, m.setStatus(fmt.Sprintf("%s is %s; cannot send to finished session", sess.ID, sess.State))
	}
	live, err := m.ctx.Tmux.HasSession(sess.ID)
	if err != nil || !live {
		return m, m.setStatus(fmt.Sprintf("%s is no longer running", sess.ID))
	}
	m.status = ""
	m.popup = NewSendPopup(sess.ID, m.theme)
	m.mode = ModePopup
	return m, m.popup.Init()
}

func (m Model) performSend(msg SendSubmitted) (Model, tea.Cmd) {
	m.mode = ModeNormal
	m.popup = nil
	if err := m.ctx.Tmux.SendKeys(msg.SessionID, msg.Text); err != nil {
		return m, m.setStatus(fmt.Sprintf("send failed: %v", err))
	}
	return m, m.setStatus(fmt.Sprintf("sent to %s", msg.SessionID))
}

// performSettingsSave normalizes the edited config (re-clamping out-of-range
// values exactly as Load would) and writes it. On success it adopts the saved
// config and closes the popup; on a write failure it leaves the popup open so
// the user can retry. The previewed values are already live in ctx.Config.
func (m Model) performSettingsSave(msg SettingsSaved) (Model, tea.Cmd) {
	cfg := msg.Config
	config.Normalize(&cfg)
	if err := config.Save(m.ctx.Paths.ConfigFile(), cfg); err != nil {
		return m, m.setStatus(fmt.Sprintf("settings save failed: %v", err))
	}
	m.ctx.Config = cfg
	themeCmd := m.applyTheme(cfg.UI.Theme)
	m.mode = ModeNormal
	m.popup = nil
	status := "settings saved"
	if n := len(cfg.Warnings); n > 0 {
		status = fmt.Sprintf("settings saved (%d value(s) adjusted)", n)
	}
	statusCmd := m.setStatus(status)
	return m, tea.Batch(statusCmd, themeCmd)
}

func (m Model) toggleMute() (Model, tea.Cmd) {
	cfg := m.ctx.Config
	cfg.Sound.Enabled = !cfg.Sound.Enabled
	if err := config.Save(m.ctx.Paths.ConfigFile(), cfg); err == nil {
		m.ctx.Config = cfg
	}
	return m, nil
}

// performSpawn executes the spawn flow when SpawnSubmitted message arrives.
func (m Model) performSpawn(s SpawnSubmitted) (Model, tea.Cmd) {
	if _, ok := m.ctx.Config.Agents[s.Agent]; !ok {
		return m, nil
	}

	lifecycle := m.ctx.NewLifecycle()
	result, err := lifecycle.Create(sessionlifecycle.CreateInput{
		Agent:               s.Agent,
		Name:                s.Name,
		Path:                s.Path,
		ProjectID:           s.ProjectID,
		AutoRegisterProject: s.ProjectID == "",
		Worktree:            &s.Worktree,
	})
	if err != nil {
		statusCmd := m.setStatus(fmt.Sprintf("spawn failed: %v", err))
		m.mode = ModeNormal
		m.popup = nil
		return m, tea.Batch(statusCmd, loadStateCmd(m.ctx))
	}
	var statusCmd tea.Cmd
	if result.Warning != nil {
		statusCmd = m.setStatus(fmt.Sprintf("spawned %s with warning: %v", result.Session.ID, result.Warning))
	}
	m.mode = ModeNormal
	m.popup = nil
	return m, tea.Batch(statusCmd, loadStateCmd(m.ctx))
}

func (m Model) performKill(target string) (Model, tea.Cmd) {
	lifecycle := m.ctx.NewLifecycle()
	result, err := lifecycle.Kill(target)
	var statusCmd tea.Cmd
	if err != nil {
		statusCmd = m.setStatus(fmt.Sprintf("kill failed: %v", err))
	} else if result.Warning != nil {
		statusCmd = m.setStatus(fmt.Sprintf("kill %s: tmux warning: %v", target, result.Warning))
	}
	m.mode = ModeNormal
	m.popup = nil
	return m, tea.Batch(statusCmd, loadStateCmd(m.ctx))
}

func (m Model) confirmPrune() (Model, tea.Cmd) {
	pid, ok := m.projectAtCursor()
	if !ok {
		return m, nil
	}
	var count int
	for _, s := range m.sessions {
		if s.ProjectID == pid && s.State.IsFinished() {
			count++
		}
	}
	if count == 0 {
		return m, m.setStatus("no finished sessions to prune")
	}
	m.status = ""
	prompt := fmt.Sprintf("prune all %d finished session(s) in %q?", count, pid)
	m.popup = NewConfirmPopup("Confirm Prune", "confirm prune", prompt, pid, confirmKindPrune, m.theme)
	m.mode = ModePopup
	return m, m.popup.Init()
}

func (m Model) performPrune(projectID string) (Model, tea.Cmd) {
	lifecycle := m.ctx.NewLifecycle()
	result, err := lifecycle.Prune(sessionlifecycle.PruneInput{
		ProjectID: projectID,
		Keep:      0,
	})
	var statusCmd tea.Cmd
	if err != nil {
		statusCmd = m.setStatus(fmt.Sprintf("prune failed: %v", err))
	} else if len(result.Warnings) > 0 {
		statusCmd = m.setStatus(fmt.Sprintf("prune: %d session(s) removed, %d warning(s)", len(result.Pruned), len(result.Warnings)))
	} else {
		statusCmd = m.setStatus(fmt.Sprintf("pruned %d session(s)", len(result.Pruned)))
	}
	m.mode = ModeNormal
	m.popup = nil
	return m, tea.Batch(statusCmd, loadStateCmd(m.ctx))
}

func (m Model) confirmRemoveProject() (Model, tea.Cmd) {
	pid, ok := m.projectAtCursor()
	if !ok {
		return m, nil
	}
	m.status = ""
	var activeCnt, finishedCnt int
	for _, s := range m.sessions {
		if s.ProjectID != pid {
			continue
		}
		if s.State.IsFinished() {
			finishedCnt++
		} else {
			activeCnt++
		}
	}
	var prompt string
	if activeCnt > 0 {
		prompt = fmt.Sprintf("remove %q? kills %d active session(s)", pid, activeCnt)
	} else {
		prompt = fmt.Sprintf("remove %q and %d session(s)?", pid, finishedCnt)
	}
	m.popup = NewConfirmPopup("Remove Project", "confirm remove", prompt, pid, confirmKindRemoveProject, m.theme)
	m.mode = ModePopup
	return m, m.popup.Init()
}

func (m Model) performRemoveProject(pid string) (Model, tea.Cmd) {
	lifecycle := m.ctx.NewLifecycle()
	result, err := lifecycle.RemoveProjectSessions(sessionlifecycle.RemoveProjectSessionsInput{
		ProjectID: pid,
		Force:     true,
	})
	var statusCmd tea.Cmd
	if err != nil {
		statusCmd = m.setStatus(fmt.Sprintf("remove failed: %v", err))
	} else if len(result.Warnings) > 0 {
		statusCmd = m.setStatus(fmt.Sprintf("removed %d session(s) with %d warning(s)", len(result.RemovedSessionIDs), len(result.Warnings)))
	}
	_ = m.ctx.Projects.Remove(pid)
	m.mode = ModeNormal
	m.popup = nil
	return m, tea.Batch(statusCmd, loadStateCmd(m.ctx))
}

func (m Model) performRename(msg RenameSubmitted) (Model, tea.Cmd) {
	lifecycle := m.ctx.NewLifecycle()
	_, err := lifecycle.Rename(msg.SessionID, msg.NewName)
	var statusCmd tea.Cmd
	if err != nil {
		statusCmd = m.setStatus(fmt.Sprintf("rename failed: %v", err))
	}
	m.mode = ModeNormal
	m.popup = nil
	return m, tea.Batch(statusCmd, loadStateCmd(m.ctx))
}
