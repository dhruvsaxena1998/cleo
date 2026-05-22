package tui

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/dhruvsaxena1998/cleo/internal/cli"
	"github.com/dhruvsaxena1998/cleo/internal/config"
	"github.com/dhruvsaxena1998/cleo/internal/events"
	"github.com/dhruvsaxena1998/cleo/internal/ids"
	"github.com/dhruvsaxena1998/cleo/internal/state"
	"github.com/dhruvsaxena1998/cleo/internal/tmux"
)

func (m Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Explicit Esc hierarchy (spec §2.2): popup -> filter -> status.
	// Intercepted at the top level so each layer behaves predictably no
	// matter which mode forwarded the keypress.
	if msg.Type == tea.KeyEsc {
		if m.mode == ModePopup && m.popup != nil {
			m.popup = nil
			m.mode = ModeNormal
			m.status = ""
			return m, nil
		}
		if m.mode == ModeFilter {
			m.mode = ModeNormal
			m.filter = ""
			return m.clampCursor(), nil
		}
		m.status = ""
		return m, nil
	}
	if m.mode == ModeFilter {
		return m.handleFilterKey(msg)
	}
	if m.mode == ModePopup && m.popup != nil {
		var cmd tea.Cmd
		m.popup, cmd = m.popup.Update(msg)
		return m, cmd
	}
	km := DefaultKeymap()
	switch {
	case key.Matches(msg, km.Quit):
		return m, tea.Quit
	case key.Matches(msg, km.Filter):
		m.status = ""
		m.mode = ModeFilter
		return m, nil
	case key.Matches(msg, km.New):
		return m.openSpawnPopup()
	case key.Matches(msg, km.View):
		return m.viewSelectedAgent()
	case key.Matches(msg, km.Enter):
		return m.attachSelectedAgent()
	case key.Matches(msg, km.Kill):
		return m.confirmKill()
	case key.Matches(msg, km.Prune):
		return m.confirmPrune()
	case key.Matches(msg, km.Remove):
		return m.confirmRemoveProject()
	case key.Matches(msg, km.Rename):
		return m.openRenamePopup()
	case key.Matches(msg, km.Mute):
		return m.toggleMute()
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
	if !m.expanded[pid] || m.cursor.agentIdx < 0 {
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
	m.status = ""
	if m.cursor.agentIdx >= 0 {
		m.cursor.agentIdx--
		if m.cursor.agentIdx < 0 {
			m.cursor.agentIdx = -1
		}
		return m, m.autoCaptureCmd()
	}
	if m.cursor.projectIdx > 0 {
		m.cursor.projectIdx--
		prevPID := m.visibleProjectIDs()[m.cursor.projectIdx]
		if m.expanded[prevPID] {
			if ss := m.sessionsFor(prevPID); len(ss) > 0 {
				m.cursor.agentIdx = len(ss) - 1
			}
		}
	}
	return m, m.autoCaptureCmd()
}

func (m Model) cursorDown() (Model, tea.Cmd) {
	m.status = ""
	pid, ok := m.projectAtCursor()
	if !ok {
		return m, nil
	}
	if m.expanded[pid] {
		ss := m.sessionsFor(pid)
		if m.cursor.agentIdx+1 < len(ss) {
			m.cursor.agentIdx++
			return m, m.autoCaptureCmd()
		}
	}
	if m.cursor.projectIdx+1 < len(m.visibleProjectIDs()) {
		m.cursor.projectIdx++
		m.cursor.agentIdx = -1
	}
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
	pid, ok := m.projectAtCursor()
	if !ok {
		return m, nil
	}
	// Status clear comes after the early return: pressing 'n' on an empty
	// row is a no-op and shouldn't clear an existing status message.
	m.status = ""
	agents := []string{}
	for k := range m.ctx.Config.Agents {
		agents = append(agents, k)
	}
	m.popup = NewSpawnPopup(pid, agents, m.theme)
	m.mode = ModePopup
	return m, m.popup.Init()
}

func (m Model) viewSelectedAgent() (Model, tea.Cmd) {
	sess, ok := m.sessionAtCursor()
	if !ok {
		return m, nil
	}
	if sess.State.IsFinished() {
		m.status = fmt.Sprintf("%s is %s; terminal is no longer attached", sess.ID, sess.State)
		return m, nil
	}
	m.selected = sess.ID
	return m, capturePaneCmd(m.ctx, sess.ID, m.ctx.Config.UI.PanePreview.Lines)
}

func (m Model) attachSelectedAgent() (Model, tea.Cmd) {
	sess, ok := m.sessionAtCursor()
	if !ok {
		return m, nil
	}
	if sess.State == state.Dead || sess.State == state.Errored {
		m.status = fmt.Sprintf("%s is %s; press K to remove it", sess.ID, sess.State)
		return m, nil
	}
	live, err := m.ctx.Tmux.HasSession(sess.ID)
	if err != nil || !live {
		_, _ = m.ctx.State.Apply(sess.ID, state.EvDead, "")
		m.status = fmt.Sprintf("%s is no longer running; marked dead", sess.ID)
		return m, loadStateCmd(m.ctx)
	}
	// Session was marked completed by idle timeout but tmux is still alive — revive it.
	if sess.State == state.Completed {
		_, _ = m.ctx.State.Apply(sess.ID, state.EvUserResume, "re-attached by user")
	}
	cliInstallFocusHooks(m.ctx)
	_ = m.ctx.Focus.Set(sess.ID, true)
	c := exec.Command("tmux", "attach", "-t", sess.ID)
	return m, tea.ExecProcess(c, func(err error) tea.Msg {
		_ = m.ctx.Focus.Set(sess.ID, false)
		// nothing to send back; just resume rendering
		return nil
	})
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
		m.status = fmt.Sprintf("%s is %s; finished sessions cannot be renamed", sess.ID, sess.State)
		return m, nil
	}
	// Clear stale status only on the success path (popup actually opens).
	m.status = ""
	m.popup = NewRenamePopup(sess.ID, sess.Name, m.theme)
	m.mode = ModePopup
	return m, m.popup.Init()
}

func (m Model) openHelpPopup() (Model, tea.Cmd) {
	m.status = ""
	m.popup = NewHelpPopup(m.theme, m.ctx.Config.Tmux.DetachKey)
	m.mode = ModePopup
	return m, m.popup.Init()
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
	agent, ok := m.ctx.Config.Agents[s.Agent]
	if !ok {
		return m, nil
	}
	var proj string
	for _, p := range m.projects {
		if p.ID == s.ProjectID {
			proj = p.Path
			break
		}
	}
	if proj == "" {
		return m, nil
	}
	existing := map[string]bool{}
	prefix := fmt.Sprintf("cleo-%s-%s-", s.ProjectID, s.Agent)
	for _, sess := range m.sessions {
		if len(sess.ID) > len(prefix) && sess.ID[:len(prefix)] == prefix {
			existing[sess.Name] = true
		}
	}
	var slug string
	if s.Name != "" {
		slug = ids.DedupeSlug(ids.Slugify(s.Name), existing)
	} else {
		slug = ids.RandomName(existing)
	}
	sid := ids.MakeSessionID(s.ProjectID, s.Agent, slug)
	_ = m.ctx.State.Put(state.Session{
		ID: sid, ProjectID: s.ProjectID, Agent: s.Agent, Name: slug, State: state.Spawning,
		StartedAt: time.Now(),
	})
	_ = m.ctx.Tmux.NewSession(tmux.NewSessionOpts{
		Name: sid, Cwd: proj, Cmd: agent.Command,
		Env: map[string]string{"CLEO_SESSION_ID": sid},
	})
	cliInstallFocusHooks(m.ctx)
	if dk := m.ctx.Config.Tmux.DetachKey; dk != "" {
		parts := strings.Fields(dk)
		if len(parts) >= 2 {
			_ = exec.Command("tmux", "bind-key", parts[len(parts)-1], "detach-client").Run()
		}
	}
	m.mode = ModeNormal
	m.popup = nil
	return m, loadStateCmd(m.ctx)
}

func (m Model) performKill(target string) (Model, tea.Cmd) {
	_ = m.ctx.Tmux.Kill(target)
	_ = m.ctx.State.Delete(target)
	m.mode = ModeNormal
	m.popup = nil
	return m, loadStateCmd(m.ctx)
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
		m.status = "no finished sessions to prune"
		return m, nil
	}
	m.status = ""
	prompt := fmt.Sprintf("prune all %d finished session(s) in %q?", count, pid)
	m.popup = NewConfirmPopup("Confirm Prune", "confirm prune", prompt, pid, confirmKindPrune, m.theme)
	m.mode = ModePopup
	return m, m.popup.Init()
}

func (m Model) performPrune(projectID string) (Model, tea.Cmd) {
	for _, s := range m.sessions {
		if s.ProjectID != projectID || !s.State.IsFinished() {
			continue
		}
		_ = events.Archive(m.ctx.Paths.EventsLog(s.ID), m.ctx.Paths.ArchiveDir())
		_ = m.ctx.State.Delete(s.ID)
	}
	m.mode = ModeNormal
	m.popup = nil
	return m, loadStateCmd(m.ctx)
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
	for _, s := range m.sessions {
		if s.ProjectID != pid {
			continue
		}
		if !s.State.IsFinished() {
			_ = m.ctx.Tmux.Kill(s.ID)
		}
		_ = m.ctx.State.Delete(s.ID)
	}
	_ = m.ctx.Projects.Remove(pid)
	m.mode = ModeNormal
	m.popup = nil
	return m, loadStateCmd(m.ctx)
}

func (m Model) performRename(msg RenameSubmitted) (Model, tea.Cmd) {
	sess, err := m.ctx.State.Get(msg.SessionID)
	if err != nil {
		m.mode = ModeNormal
		m.popup = nil
		return m, nil
	}
	sess.Name = ids.Slugify(msg.NewName)
	_ = m.ctx.State.Put(sess)
	m.mode = ModeNormal
	m.popup = nil
	return m, loadStateCmd(m.ctx)
}

func cliInstallFocusHooks(c *cli.Ctx) {
	installer, ok := c.Tmux.(cli.TmuxFocusInstaller)
	if !ok {
		return
	}
	cleoBin, err := os.Executable()
	if err != nil {
		return
	}
	cleoBin, _ = filepath.Abs(cleoBin)
	_ = installer.InstallFocusHooks(cleoBin)
}
