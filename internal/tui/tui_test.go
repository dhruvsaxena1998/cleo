package tui

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/exp/teatest"

	"github.com/dhruvsaxena1998/cleo/internal/cli"
	"github.com/dhruvsaxena1998/cleo/internal/projects"
	"github.com/dhruvsaxena1998/cleo/internal/state"
	"github.com/dhruvsaxena1998/cleo/internal/tmux"
)

// newTestCtx returns a *cli.Ctx wired against a temp config root with the fake
// tmux double — handy for unit tests that exercise Update without driving a
// full teatest harness.
func newTestCtx(t *testing.T) *cli.Ctx {
	t.Helper()
	root := t.TempDir()
	c, err := cli.NewCtxWithRoot(root)
	if err != nil {
		t.Fatal(err)
	}
	usePortableAgentCommand(c, "claude")
	usePortableAgentCommand(c, "codex")
	usePortableAgentCommand(c, "opencode")
	usePortableAgentCommand(c, "pi")
	c.Tmux = &fakeTmux{live: map[string]bool{}}
	return c
}

func mkdirAll(p string) error { return os.MkdirAll(p, 0o755) }

func usePortableAgentCommand(c *cli.Ctx, agentName string) {
	agent := c.Config.Agents[agentName]
	agent.Command = "sh"
	c.Config.Agents[agentName] = agent
}

func contains(b []byte, s string) bool {
	return strings.Contains(string(b), s)
}

// fakeTmux is a test double for cli.TmuxClient that reports a fixed set of live sessions.
type fakeTmux struct {
	live          map[string]bool
	capturedLines []int
	newSessionErr error
	attached      []string
}

func (f *fakeTmux) NewSession(o tmux.NewSessionOpts) error {
	if f.newSessionErr != nil {
		return f.newSessionErr
	}
	if f.live == nil {
		f.live = map[string]bool{}
	}
	f.live[o.Name] = true
	return nil
}
func (f *fakeTmux) HasSession(n string) (bool, error) { return f.live[n], nil }
func (f *fakeTmux) LsPrefix(prefix string) ([]string, error) {
	var out []string
	for n := range f.live {
		if strings.HasPrefix(n, prefix) {
			out = append(out, n)
		}
	}
	return out, nil
}
func (f *fakeTmux) Kill(n string) error            { delete(f.live, n); return nil }
func (f *fakeTmux) BindDetachKey(string) error     { return nil }
func (f *fakeTmux) InstallFocusHooks(string) error { return nil }
func (f *fakeTmux) CapturePane(_ string, lines int) (string, error) {
	f.capturedLines = append(f.capturedLines, lines)
	return "", nil
}
func (f *fakeTmux) SendKeys(name string, text string) error { return nil }
func (f *fakeTmux) RenameSession(from, to string) error     { return nil }
func (f *fakeTmux) SessionPIDs(name string) ([]int, error)  { return nil, nil }
func (f *fakeTmux) AttachCmd(sessionID string) *exec.Cmd {
	f.attached = append(f.attached, sessionID)
	return exec.Command("true") // harmless no-op; records the attach request
}

type fakeEditorLauncher struct {
	started []*exec.Cmd
	err     error
}

func (f *fakeEditorLauncher) StartDetached(cmd *exec.Cmd) error {
	f.started = append(f.started, cmd)
	return f.err
}

func TestOpenEditorKeyOnProjectRowStartsDetachedEditorForProjectPath(t *testing.T) {
	c := newTestCtx(t)
	c.Config.UI.Editor = "code --reuse-window"
	target := filepath.Join(t.TempDir(), "myapp")
	if err := mkdirAll(target); err != nil {
		t.Fatal(err)
	}
	if _, err := c.Projects.Add(target); err != nil {
		t.Fatal(err)
	}
	m := New(c)
	m.projects, _ = c.Projects.List()
	m.cursor.projectIdx = 0
	m.cursor.agentIdx = -1
	launcher := &fakeEditorLauncher{}
	m.editorLauncher = launcher

	gotModel, cmd := m.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("e")})
	got := gotModel.(Model)

	if len(launcher.started) != 1 {
		t.Fatalf("started detached editors = %d, want 1", len(launcher.started))
	}
	if !containsEnv(launcher.started[0].Env, "CLEO_PROJECT_PATH="+target) {
		t.Fatalf("project path env missing: %v", launcher.started[0].Env)
	}
	if !strings.Contains(got.status, "opening Project myapp") {
		t.Fatalf("status = %q", got.status)
	}
	if cmd == nil {
		t.Fatal("successful detached launch should schedule status expiry")
	}
}

func TestOpenEditorCtrlGKeyStartsDetachedEditorForProjectPath(t *testing.T) {
	c := newTestCtx(t)
	c.Config.UI.Editor = "code"
	target := filepath.Join(t.TempDir(), "myapp")
	if err := mkdirAll(target); err != nil {
		t.Fatal(err)
	}
	if _, err := c.Projects.Add(target); err != nil {
		t.Fatal(err)
	}
	m := New(c)
	m.projects, _ = c.Projects.List()
	m.cursor.projectIdx = 0
	m.cursor.agentIdx = -1
	launcher := &fakeEditorLauncher{}
	m.editorLauncher = launcher

	_, _ = m.handleKey(tea.KeyMsg{Type: tea.KeyCtrlG})

	if len(launcher.started) != 1 {
		t.Fatalf("started detached editors = %d, want 1", len(launcher.started))
	}
	if !containsEnv(launcher.started[0].Env, "CLEO_PROJECT_PATH="+target) {
		t.Fatalf("project path env missing: %v", launcher.started[0].Env)
	}
}

func TestOpenEditorKeyOnFinishedSessionTargetsParentProjectPath(t *testing.T) {
	c := newTestCtx(t)
	c.Config.UI.Editor = "code"
	target := filepath.Join(t.TempDir(), "myapp")
	if err := mkdirAll(target); err != nil {
		t.Fatal(err)
	}
	if _, err := c.Projects.Add(target); err != nil {
		t.Fatal(err)
	}
	_ = c.State.Put(state.Session{
		ID: "cleo-myapp-codex-1", ProjectID: "myapp", Agent: "codex",
		Name: "1", State: state.Dead, StartedAt: time.Now(), LastEventAt: time.Now(),
	})
	m := New(c)
	m.projects, _ = c.Projects.List()
	m.sessions, _ = c.State.List()
	m.expanded["myapp"] = true
	m.cursor.projectIdx = 0
	m.cursor.agentIdx = 0
	launcher := &fakeEditorLauncher{}
	m.editorLauncher = launcher

	_, _ = m.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("e")})

	if len(launcher.started) != 1 {
		t.Fatalf("started detached editors = %d, want 1", len(launcher.started))
	}
	if !containsEnv(launcher.started[0].Env, "CLEO_PROJECT_PATH="+target) {
		t.Fatalf("project path env missing: %v", launcher.started[0].Env)
	}
}

func TestOpenEditorKeyReportsMissingAndUnsupportedEditors(t *testing.T) {
	t.Setenv("EDITOR", "")
	c := newTestCtx(t)
	target := filepath.Join(t.TempDir(), "myapp")
	if err := mkdirAll(target); err != nil {
		t.Fatal(err)
	}
	if _, err := c.Projects.Add(target); err != nil {
		t.Fatal(err)
	}
	m := New(c)
	m.projects, _ = c.Projects.List()
	m.cursor.projectIdx = 0
	m.cursor.agentIdx = -1

	gotModel, cmd := m.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("e")})
	got := gotModel.(Model)
	if cmd != nil {
		t.Fatal("missing editor should not return a command")
	}
	if !strings.Contains(got.status, "no editor configured") {
		t.Fatalf("missing editor status = %q", got.status)
	}

	got.ctx.Config.UI.Editor = "mystery-editor"
	gotModel, cmd = got.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("e")})
	got = gotModel.(Model)
	if cmd != nil {
		t.Fatal("unsupported editor should not return a command")
	}
	if !strings.Contains(got.status, "unsupported editor") {
		t.Fatalf("unsupported editor status = %q", got.status)
	}
}

func TestOpenEditorKeyReportsDetachedLaunchFailure(t *testing.T) {
	c := newTestCtx(t)
	c.Config.UI.Editor = "code"
	target := filepath.Join(t.TempDir(), "myapp")
	if err := mkdirAll(target); err != nil {
		t.Fatal(err)
	}
	if _, err := c.Projects.Add(target); err != nil {
		t.Fatal(err)
	}
	m := New(c)
	m.projects, _ = c.Projects.List()
	m.cursor.projectIdx = 0
	m.cursor.agentIdx = -1
	m.editorLauncher = &fakeEditorLauncher{err: errors.New("boom")}

	gotModel, cmd := m.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("e")})
	got := gotModel.(Model)
	if cmd != nil {
		t.Fatal("failed launch should not schedule status expiry")
	}
	if !strings.Contains(got.status, "open editor failed: boom") {
		t.Fatalf("status = %q", got.status)
	}
}

func TestOpenEditorKeyWithTerminalEditorReturnsProcessCommand(t *testing.T) {
	c := newTestCtx(t)
	c.Config.UI.Editor = "nvim -p"
	target := filepath.Join(t.TempDir(), "myapp")
	if err := mkdirAll(target); err != nil {
		t.Fatal(err)
	}
	if _, err := c.Projects.Add(target); err != nil {
		t.Fatal(err)
	}
	m := New(c)
	m.projects, _ = c.Projects.List()
	m.cursor.projectIdx = 0
	m.cursor.agentIdx = -1
	launcher := &fakeEditorLauncher{}
	m.editorLauncher = launcher

	_, cmd := m.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("e")})
	if cmd == nil {
		t.Fatal("terminal editor should return a process command")
	}
	if len(launcher.started) != 0 {
		t.Fatalf("terminal editor should not launch detached, got %d starts", len(launcher.started))
	}
}

func containsEnv(env []string, want string) bool {
	for _, e := range env {
		if e == want {
			return true
		}
	}
	return false
}

func TestRenamePopupOpensAndUpdatesSessionName(t *testing.T) {
	root := t.TempDir()
	c, err := cli.NewCtxWithRoot(root)
	if err != nil {
		t.Fatal(err)
	}
	c.Tmux = &fakeTmux{live: map[string]bool{"cleo-myapp-claude-1": true}}
	target := filepath.Join(t.TempDir(), "myapp")
	_ = os.MkdirAll(target, 0o755)
	if _, err := c.Projects.Add(target); err != nil {
		t.Fatal(err)
	}
	_ = c.State.Put(state.Session{
		ID: "cleo-myapp-claude-1", ProjectID: "myapp", Agent: "claude",
		Name: "1", State: state.Running, StartedAt: time.Now(),
	})

	m := New(c)
	tm := teatest.NewTestModel(t, m, teatest.WithInitialTermSize(120, 40))

	// Wait for session to render, then navigate to it and press r
	teatest.WaitFor(t, tm.Output(), func(b []byte) bool {
		return contains(b, "cl") && contains(b, "run")
	}, teatest.WithDuration(3*time.Second))

	// Navigate down to the session row (project is expanded with one session)
	tm.Send(tea.KeyMsg{Type: tea.KeyDown})
	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("r")})

	// Rename popup should appear
	teatest.WaitFor(t, tm.Output(), func(b []byte) bool {
		return contains(b, "Rename Session")
	}, teatest.WithDuration(2*time.Second))

	// Clear pre-filled name, type new name, confirm
	tm.Send(tea.KeyMsg{Type: tea.KeyCtrlU}) // kill line — clears the text input
	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("my-task")})
	tm.Send(tea.KeyMsg{Type: tea.KeyEnter})

	// Verify the session name was updated in the store
	teatest.WaitFor(t, tm.Output(), func(b []byte) bool {
		sess, err := c.State.Get("cleo-myapp-claude-1")
		return err == nil && sess.Name == "my-task"
	}, teatest.WithDuration(2*time.Second))

	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})
	tm.WaitFinished(t)
}

func TestSidebarRendersProjectsAndSessions(t *testing.T) {
	root := t.TempDir()
	c, err := cli.NewCtxWithRoot(root)
	if err != nil {
		t.Fatal(err)
	}
	// Use a fake tmux so reconcile.Run sees the session as live and doesn't mark it Dead.
	c.Tmux = &fakeTmux{live: map[string]bool{"cleo-myapp-claude-1": true}}
	target := filepath.Join(t.TempDir(), "myapp")
	if err := mkdirAll(target); err != nil {
		t.Fatal(err)
	}
	if _, err := c.Projects.Add(target); err != nil {
		t.Fatal(err)
	}
	_ = c.State.Put(state.Session{
		ID: "cleo-myapp-claude-1", ProjectID: "myapp", Agent: "claude",
		Name: "1", State: state.Running, StartedAt: time.Now(),
	})

	m := New(c)
	tm := teatest.NewTestModel(t, m, teatest.WithInitialTermSize(120, 40))

	// Auto-expand fires on first state load; just wait for agent and state to appear.
	teatest.WaitFor(t, tm.Output(), func(b []byte) bool {
		return contains(b, "cl") && contains(b, "run")
	}, teatest.WithDuration(3*time.Second))

	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})
	tm.WaitFinished(t)
}

func TestEnterOnDeadSessionDoesNotAttach(t *testing.T) {
	root := t.TempDir()
	c, err := cli.NewCtxWithRoot(root)
	if err != nil {
		t.Fatal(err)
	}
	c.Tmux = &fakeTmux{live: map[string]bool{}}
	target := filepath.Join(t.TempDir(), "myapp")
	if err := mkdirAll(target); err != nil {
		t.Fatal(err)
	}
	if _, err := c.Projects.Add(target); err != nil {
		t.Fatal(err)
	}
	_ = c.State.Put(state.Session{
		ID: "cleo-myapp-codex-1", ProjectID: "myapp", Agent: "codex",
		Name: "1", State: state.Dead, StartedAt: time.Now(), LastEventAt: time.Now(),
	})

	m := New(c)
	m.projects, _ = c.Projects.List()
	m.sessions, _ = c.State.List()
	m.expanded["myapp"] = true
	m.cursor.projectIdx = 0
	m.cursor.agentIdx = 0

	got, cmd := m.attachSelectedAgent()
	if cmd != nil {
		t.Fatal("dead session should not produce an attach command")
	}
	if !strings.Contains(got.status, "press K") {
		t.Fatalf("expected remove hint status, got %q", got.status)
	}
}

// TestEnterOnLiveSessionAttachesViaSeam is the live-session counterpart to the
// dead-session test: pressing Enter on a ready Session must request the attach
// plan from lifecycle.Attach (which sets focus and builds the command via the
// Tmux seam) and run the returned command — not hand-build a raw tmux command.
// Focus-clear on detach is pinned at the lifecycle level (TestAttachDoneClearsFocus);
// here it lives in the opaque ExecProcess callback.
func TestEnterOnLiveSessionAttachesViaSeam(t *testing.T) {
	root := t.TempDir()
	c, err := cli.NewCtxWithRoot(root)
	if err != nil {
		t.Fatal(err)
	}
	sid := "cleo-myapp-claude-1"
	fake := &fakeTmux{live: map[string]bool{sid: true}}
	c.Tmux = fake
	target := filepath.Join(t.TempDir(), "myapp")
	if err := mkdirAll(target); err != nil {
		t.Fatal(err)
	}
	if _, err := c.Projects.Add(target); err != nil {
		t.Fatal(err)
	}
	_ = c.State.Put(state.Session{
		ID: sid, ProjectID: "myapp", Agent: "claude",
		Name: "1", State: state.Running, StartedAt: time.Now(),
	})

	m := New(c)
	m.projects, _ = c.Projects.List()
	m.sessions, _ = c.State.List()
	m.expanded["myapp"] = true
	m.cursor.projectIdx = 0
	m.cursor.agentIdx = 0

	_, cmd := m.attachSelectedAgent()
	if cmd == nil {
		t.Fatal("live session should produce an attach command")
	}
	if len(fake.attached) != 1 || fake.attached[0] != sid {
		t.Fatalf("expected attach requested for %q via the seam, got %v", sid, fake.attached)
	}
	// The verb sets focus on before handing back the command; the TUI clears it
	// in the ExecProcess detach callback (plan.Done).
	if !c.Focus.IsFocused(sid) {
		t.Fatal("expected focus set after attach routed through lifecycle.Attach")
	}
}

// TestPreviewTickAlwaysReArms locks in the self-recovering preview ticker:
// every previewTickMsg fire must return a non-nil command that produces another
// previewTickMsg, regardless of whether a capture was dispatched. The previous
// chain (paneCapturedMsg -> tea.Tick -> capturePaneTickMsg) deadlocked when
// the user navigated mid-flight, because the tick was scheduled from the
// *response* path; if the response sid no longer matched the selection, the
// loop returned m, nil and never fired again until manual nav.
func TestPreviewTickAlwaysReArms(t *testing.T) {
	c := newTestCtx(t)
	c.Config.UI.PanePreview.Interval = 10 * time.Millisecond
	m := New(c)
	m.projects = []projects.Project{{ID: "p"}}
	m.sessions = []state.Session{{ID: "s1", State: state.Running, ProjectID: "p"}}
	m.cursor.projectIdx = 0
	m.cursor.agentIdx = 0
	m.expanded = map[string]bool{"p": true}

	// First tick: dispatches a capture and re-arms. Mirrors first paint after
	// the user lands on a session.
	updated, cmd := m.Update(previewTickMsg{})
	if cmd == nil {
		t.Fatal("expected non-nil cmd from previewTickMsg")
	}
	out := runCmdAndCollect(t, cmd, 200*time.Millisecond)
	if !containsType(out, previewTickMsg{}) {
		t.Fatalf("expected previewTickMsg in first-tick output, got %v", out)
	}
	m = updated.(Model)
	if !m.paneCaptureInFlight {
		t.Fatal("expected paneCaptureInFlight=true after first tick dispatched capture")
	}

	// Second tick *while a capture is still in flight* — this is the
	// navigation-mid-flight scenario that broke v0.1's chain. The new ticker
	// must still re-arm (no capture dispatched, no deadlock).
	updated, cmd = m.Update(previewTickMsg{})
	if cmd == nil {
		t.Fatal("in-flight tick must still re-arm — got nil cmd (deadlock!)")
	}
	out = runCmdAndCollect(t, cmd, 200*time.Millisecond)
	if !containsType(out, previewTickMsg{}) {
		t.Errorf("expected previewTickMsg even when capture in-flight, got %v", out)
	}
	// Capture response clears the in-flight flag, unblocking future captures.
	updated, _ = updated.(Model).Update(paneCapturedMsg{sid: "s1", content: "hello"})
	if updated.(Model).paneCaptureInFlight {
		t.Error("paneCapturedMsg should clear paneCaptureInFlight")
	}
}

func TestPanePreviewDisabledSkipsCapture(t *testing.T) {
	c := newTestCtx(t)
	c.Config.UI.PanePreview.Enabled = false
	fake := c.Tmux.(*fakeTmux)
	m := New(c)
	m.projects = []projects.Project{{ID: "p"}}
	m.sessions = []state.Session{{ID: "s1", State: state.Running, ProjectID: "p"}}
	m.cursor.projectIdx = 0
	m.cursor.agentIdx = 0
	m.expanded = map[string]bool{"p": true}

	if cmd := m.autoCaptureCmd(); cmd != nil {
		t.Fatal("disabled pane preview should not produce capture command")
	}
	if len(fake.capturedLines) != 0 {
		t.Fatalf("unexpected captures: %v", fake.capturedLines)
	}
}

func TestPanePreviewUsesConfiguredLineCount(t *testing.T) {
	c := newTestCtx(t)
	c.Config.UI.PanePreview.Lines = 7
	fake := c.Tmux.(*fakeTmux)
	m := New(c)
	m.projects = []projects.Project{{ID: "p"}}
	m.sessions = []state.Session{{ID: "s1", State: state.Running, ProjectID: "p"}}
	m.cursor.projectIdx = 0
	m.cursor.agentIdx = 0
	m.expanded = map[string]bool{"p": true}

	cmd := m.autoCaptureCmd()
	if cmd == nil {
		t.Fatal("expected capture command")
	}
	_ = cmd()
	if !reflect.DeepEqual(fake.capturedLines, []int{7}) {
		t.Fatalf("captured lines = %v", fake.capturedLines)
	}
}

// TestFooterHidesViewHintWhenPreviewEnabled locks in the keybind-relevance fix:
// the "v" (manual capture) hint is redundant when the preview panel already
// auto-refreshes, so it only appears for a running session when pane preview is
// disabled.
func TestFooterHidesViewHintWhenPreviewEnabled(t *testing.T) {
	mkModel := func(previewEnabled bool) Model {
		c := newTestCtx(t)
		c.Config.UI.PanePreview.Enabled = previewEnabled
		m := New(c)
		m.projects = []projects.Project{{ID: "p"}}
		m.sessions = []state.Session{{ID: "s1", State: state.Running, ProjectID: "p"}}
		m.cursor.projectIdx = 0
		m.cursor.agentIdx = 0
		m.expanded = map[string]bool{"p": true}
		m.width = 120
		m.height = 40
		return m
	}

	if footer := mkModel(true).renderFooter(120); strings.Contains(footer, "view") {
		t.Errorf("preview enabled: footer should omit view hint, got %q", footer)
	}
	if footer := mkModel(false).renderFooter(120); !strings.Contains(footer, "view") {
		t.Errorf("preview disabled: footer should include view hint, got %q", footer)
	}
}

func TestConfigWarningsShowStartupStatus(t *testing.T) {
	c := newTestCtx(t)
	c.Config.Warnings = []string{"sound.volume above 1; clamped to 1"}
	m := New(c)

	if cmd := m.Init(); cmd == nil {
		t.Fatal("expected init command")
	}
	if m.status != "config warnings: run cleo doctor" {
		t.Fatalf("status = %q", m.status)
	}
}

func TestSpawnFailureStatusRendersInFooter(t *testing.T) {
	c := newTestCtx(t)
	target := filepath.Join(t.TempDir(), "myapp")
	if err := mkdirAll(target); err != nil {
		t.Fatal(err)
	}
	registered, err := c.Projects.Add(target)
	if err != nil {
		t.Fatal(err)
	}
	agent := c.Config.Agents["codex"]
	agent.Command = "zzcleo"
	c.Config.Agents["codex"] = agent

	m := New(c)
	m.projects, _ = c.Projects.List()
	m.width = 120
	m.height = 40

	updated, _ := m.performSpawn(SpawnSubmitted{
		ProjectID: registered.ID,
		Path:      target,
		Agent:     "codex",
		Name:      "will-fail",
	})

	view := updated.View()
	if !strings.Contains(view, "agent command for \"codex\"") || !strings.Contains(view, "not found in PATH") {
		t.Fatalf("spawn failure status should render in footer, status=%q view=%q", updated.status, view)
	}
}

func TestSpawnRollsBackStateWhenTmuxCreationFails(t *testing.T) {
	c := newTestCtx(t)
	target := filepath.Join(t.TempDir(), "myapp")
	if err := mkdirAll(target); err != nil {
		t.Fatal(err)
	}
	if _, err := c.Projects.Add(target); err != nil {
		t.Fatal(err)
	}
	c.Tmux = &fakeTmux{
		live:          map[string]bool{},
		newSessionErr: errors.New("tmux refused session"),
	}

	m := New(c)
	m.projects, _ = c.Projects.List()
	m.sessions, _ = c.State.List()
	m.mode = ModePopup
	m.popup = NewSpawnPopup("myapp", m.projects, "/tmp", []string{"claude"}, m.theme)

	updated, _ := m.performSpawn(SpawnSubmitted{
		ProjectID: "myapp",
		Path:      target,
		Agent:     "claude",
		Name:      "will-fail",
	})

	if _, err := c.State.Get("cleo-myapp-claude-will-fail"); !errors.Is(err, state.ErrSessionNotFound) {
		t.Fatalf("failed spawn should roll back state, got err=%v", err)
	}
	if updated.status == "" || !strings.Contains(updated.status, "tmux refused session") {
		t.Fatalf("expected spawn failure status, got %q", updated.status)
	}
	if updated.mode != ModeNormal || updated.popup != nil {
		t.Fatalf("spawn failure should close popup, mode=%v popup=%#v", updated.mode, updated.popup)
	}
}

// runCmdAndCollect runs cmd to completion (or timeout) and returns each tea.Msg
// produced. tea.BatchMsg is unwrapped one level — children are invoked once and
// their messages collected. Use it to assert the *shape* of a Bubble Tea cmd
// chain without driving a full Program.
func runCmdAndCollect(t *testing.T, cmd tea.Cmd, timeout time.Duration) []tea.Msg {
	t.Helper()
	if cmd == nil {
		return nil
	}
	ch := make(chan tea.Msg, 8)
	go func() {
		msg := cmd()
		if batch, ok := msg.(tea.BatchMsg); ok {
			for _, sub := range batch {
				if sub != nil {
					ch <- sub()
				}
			}
		} else {
			ch <- msg
		}
		close(ch)
	}()
	var out []tea.Msg
	deadline := time.After(timeout)
	for {
		select {
		case mv, ok := <-ch:
			if !ok {
				return out
			}
			out = append(out, mv)
		case <-deadline:
			return out
		}
	}
}

func containsType(msgs []tea.Msg, want tea.Msg) bool {
	for _, m := range msgs {
		if reflect.TypeOf(m) == reflect.TypeOf(want) {
			return true
		}
	}
	return false
}

// visualWidth ignores ANSI sequences when measuring terminal cell width.
func visualWidth(s string) int { return lipgloss.Width(s) }

// TestPreviewLinesAreTruncatedToPanelWidth pins the v0.2 fix for Bug D — long
// captured lines used to wrap and shove the panel border off-screen. The
// preview body now truncates each line to the panel inner width.
func TestPreviewLinesAreTruncatedToPanelWidth(t *testing.T) {
	c := newTestCtx(t)
	m := New(c)
	long := strings.Repeat("X", 200)
	m.paneCache = map[string]string{"s1": long + "\nshort\n"}
	m.sessions = []state.Session{{ID: "s1", State: state.Running, ProjectID: "p"}}
	m.projects = []projects.Project{{ID: "p"}}
	m.expanded = map[string]bool{"p": true}
	m.cursor.projectIdx = 0
	m.cursor.agentIdx = 0
	m.width, m.height = 80, 30

	out := m.renderPreviewPanel(40, 20, m.sessions[0], true)
	for _, line := range strings.Split(out, "\n") {
		if visualWidth(line) > 40 {
			t.Errorf("line wider than panel: %q (%d cells)", line, visualWidth(line))
		}
	}
}

// TestPreviewWhitespaceShowsAttachHint covers the v0.2 fix for Bug E — when
// the captured pane is non-empty but only whitespace (e.g. an agent that
// rendered nothing yet, or one launched with --no-attach), the preview now
// nudges the user to press Enter instead of pretending it's still loading.
func TestPreviewWhitespaceShowsAttachHint(t *testing.T) {
	c := newTestCtx(t)
	m := New(c)
	m.paneCache = map[string]string{"s1": "   \n  \n"}
	sess := state.Session{ID: "s1", State: state.Running}
	out := m.renderPreviewPanel(60, 10, sess, true)
	if !strings.Contains(out, "press Enter to attach") {
		t.Errorf("expected attach hint for whitespace-only pane, got: %q", out)
	}
}

// TestPreviewEmptyShowsLoading complements the whitespace test: a missing
// cache entry (capture not landed yet) keeps the original "loading…" hint.
func TestPreviewEmptyShowsLoading(t *testing.T) {
	c := newTestCtx(t)
	m := New(c)
	m.paneCache = map[string]string{}
	sess := state.Session{ID: "s1", State: state.Running}
	out := m.renderPreviewPanel(60, 10, sess, true)
	if !strings.Contains(out, "loading") {
		t.Errorf("expected loading hint for empty cache, got: %q", out)
	}
}

// TestPaneCapturedMsgSkipsCacheUpdateOnUnchangedContent locks in the
// content-diff optimization for issue #34: paneCache must NOT be updated
// when the captured content is identical, so Bubble Tea's output dedup can
// skip the preview repaint entirely.
func TestPaneCapturedMsgSkipsCacheUpdateOnUnchangedContent(t *testing.T) {
	c := newTestCtx(t)
	m := New(c)
	m.paneCache = map[string]string{"s1": "unchanged content"}
	m.sessions = []state.Session{{ID: "s1", State: state.Running, ProjectID: "p"}}
	m.projects = []projects.Project{{ID: "p"}}
	m.expanded = map[string]bool{"p": true}
	m.cursor.projectIdx = 0
	m.cursor.agentIdx = 0
	m.paneCaptureInFlight = true

	// Send a paneCapturedMsg with the SAME content that's already cached.
	m2, _ := m.Update(paneCapturedMsg{sid: "s1", content: "unchanged content"})
	updated := m2.(Model)

	// paneCache must NOT be replaced (same value means the map entry is
	// untouched, so Bubble Tea's render dedup sees identical View output).
	if got := updated.paneCache["s1"]; got != "unchanged content" {
		t.Errorf("paneCache should preserve old value, got %q", got)
	}
	// paneCaptureInFlight must still be cleared (critical for ticker health).
	if updated.paneCaptureInFlight {
		t.Error("paneCaptureInFlight must be cleared even on unchanged content")
	}

	// Now send DIFFERENT content — cache must update.
	m3, _ := updated.Update(paneCapturedMsg{sid: "s1", content: "new content"})
	updated2 := m3.(Model)
	if got := updated2.paneCache["s1"]; got != "new content" {
		t.Errorf("paneCache should update on content change, got %q", got)
	}
}

// updateAsModel runs Update and asserts the resulting tea.Model is a Model.
// Used by the Esc-hierarchy and status-clear tests below.
func updateAsModel(m Model, msg tea.Msg) Model {
	out, _ := m.Update(msg)
	return out.(Model)
}

// TestEscClosesPopupOnly locks in step 1 of the Esc hierarchy: when a popup
// is open, Esc closes the popup and clears the status line.
func TestEscClosesPopupOnly(t *testing.T) {
	c := newTestCtx(t)
	m := New(c)
	m.popup = NewHelpPopup(m.theme, "")
	m.mode = ModePopup
	m.status = "stale-status"

	m2 := updateAsModel(m, tea.KeyMsg{Type: tea.KeyEsc})
	if m2.popup != nil {
		t.Error("popup should be closed")
	}
	if m2.mode != ModeNormal {
		t.Errorf("mode: want Normal, got %v", m2.mode)
	}
	if m2.status != "" {
		t.Errorf("status should be cleared, got %q", m2.status)
	}
}

// TestEscInNormalClearsStatus locks in step 2 of the Esc hierarchy: with no
// popup active, Esc just clears any stale status line message.
func TestEscInNormalClearsStatus(t *testing.T) {
	c := newTestCtx(t)
	m := New(c)
	m.mode = ModeNormal
	m.status = "old"

	m2 := updateAsModel(m, tea.KeyMsg{Type: tea.KeyEsc})
	if m2.status != "" {
		t.Errorf("status: want empty, got %q", m2.status)
	}
}

func TestQuickMessageStatusExpires(t *testing.T) {
	c := newTestCtx(t)
	m := New(c)

	m2, cmd := m.performSend(SendSubmitted{SessionID: "s1", Text: "hello"})
	if cmd == nil {
		t.Fatal("performSend should schedule status expiry")
	}
	if m2.status != "sent to s1" {
		t.Fatalf("status = %q, want sent confirmation", m2.status)
	}

	updated, _ := m2.Update(statusExpiredMsg{id: m2.statusTimerID})
	got := updated.(Model)
	if got.status != "" {
		t.Fatalf("status should expire, got %q", got.status)
	}
}

func TestStaleQuickMessageStatusExpiryDoesNotClearNewerStatus(t *testing.T) {
	c := newTestCtx(t)
	m := New(c)

	m2, _ := m.performSend(SendSubmitted{SessionID: "s1", Text: "hello"})
	staleID := m2.statusTimerID
	m3, _ := m2.performSend(SendSubmitted{SessionID: "s1", Text: "again"})

	updated, _ := m3.Update(statusExpiredMsg{id: staleID})
	got := updated.(Model)
	if got.status != "sent to s1" {
		t.Fatalf("stale expiry cleared newer status, got %q", got.status)
	}
}

func TestQuickMessageStatusClearsAndInvalidatesOnNavigation(t *testing.T) {
	c := newTestCtx(t)
	m := New(c)
	m.projects = []projects.Project{{ID: "p1"}}
	m.sessions = []state.Session{
		{ID: "s1", ProjectID: "p1", State: state.Running},
		{ID: "s2", ProjectID: "p1", State: state.Running},
	}
	m.expanded = map[string]bool{"p1": true}
	m.cursor.projectIdx = 0
	m.cursor.agentIdx = 0

	m2, _ := m.performSend(SendSubmitted{SessionID: "s1", Text: "hello"})
	staleID := m2.statusTimerID
	m3, _ := m2.cursorDown()
	if m3.status != "" {
		t.Fatalf("navigation should clear quick message status, got %q", m3.status)
	}

	m3.status = "new status"
	updated, _ := m3.Update(statusExpiredMsg{id: staleID})
	got := updated.(Model)
	if got.status != "new status" {
		t.Fatalf("stale expiry cleared status after navigation, got %q", got.status)
	}
}

// TestStatusClearsOnExpand locks in v0.1 behavior: toggleExpand wipes any
// stale status line so the next user-initiated state change starts clean.
func TestStatusClearsOnExpand(t *testing.T) {
	c := newTestCtx(t)
	m := New(c)
	m.projects = []projects.Project{{ID: "p1"}}
	m.cursor.projectIdx = 0
	m.status = "old"

	m2, _ := m.toggleExpand()
	if m2.status != "" {
		t.Errorf("status should clear on expand, got %q", m2.status)
	}
}

// TestStatusClearsOnPopupOpen covers spec §2.2 across ALL four popup
// openers, not just help. Each must clear any stale status line on the
// success path (where the popup actually opens).
func TestStatusClearsOnPopupOpen(t *testing.T) {
	cases := []struct {
		name string
		// open returns the post-action Model; setup seeds projects/sessions
		// so the opener finds something at the cursor and doesn't early-return.
		setup func(*Model)
		open  func(Model) (Model, tea.Cmd)
	}{
		{
			name:  "help",
			setup: func(_ *Model) {},
			open:  func(m Model) (Model, tea.Cmd) { return m.openHelpPopup() },
		},
		{
			name: "spawn",
			setup: func(m *Model) {
				m.projects = []projects.Project{{ID: "p1"}}
				m.cursor.projectIdx = 0
			},
			open: func(m Model) (Model, tea.Cmd) { return m.openSpawnPopup() },
		},
		{
			name: "kill",
			setup: func(m *Model) {
				m.projects = []projects.Project{{ID: "p1"}}
				m.sessions = []state.Session{{ID: "s1", ProjectID: "p1", State: state.Running}}
				m.expanded = map[string]bool{"p1": true}
				m.cursor.projectIdx = 0
				m.cursor.agentIdx = 0
			},
			open: func(m Model) (Model, tea.Cmd) { return m.confirmKill() },
		},
		{
			name: "rename",
			setup: func(m *Model) {
				m.projects = []projects.Project{{ID: "p1"}}
				m.sessions = []state.Session{{ID: "s1", ProjectID: "p1", State: state.Running}}
				m.expanded = map[string]bool{"p1": true}
				m.cursor.projectIdx = 0
				m.cursor.agentIdx = 0
			},
			open: func(m Model) (Model, tea.Cmd) { return m.openRenamePopup() },
		},
		{
			name: "send",
			setup: func(m *Model) {
				m.projects = []projects.Project{{ID: "p1"}}
				m.sessions = []state.Session{{ID: "s1", ProjectID: "p1", State: state.Running}}
				m.expanded = map[string]bool{"p1": true}
				m.cursor.projectIdx = 0
				m.cursor.agentIdx = 0
				// openSendPopup guards against missing tmux sessions; mark it live.
				fake := m.ctx.Tmux.(*fakeTmux)
				fake.live["s1"] = true
			},
			open: func(m Model) (Model, tea.Cmd) { return m.openSendPopup() },
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := newTestCtx(t)
			m := New(c)
			tc.setup(&m)
			m.status = "old"

			m2, _ := tc.open(m)
			if m2.mode != ModePopup {
				t.Fatalf("%s opener should enter ModePopup, got %v", tc.name, m2.mode)
			}
			if m2.status != "" {
				t.Errorf("%s opener should clear status, got %q", tc.name, m2.status)
			}
		})
	}
}

// TestCursorUpDownNavigation locks in symmetric up/down navigation across
// project headers and session rows (issue #24). Uses direct method calls —
// no teatest harness required.
func TestCursorUpDownNavigation(t *testing.T) {
	c := newTestCtx(t)
	m := New(c)
	m.projects = []projects.Project{{ID: "p1"}, {ID: "p2"}}
	now := time.Now()
	m.sessions = []state.Session{
		{ID: "s1a", ProjectID: "p1", StartedAt: now},
		{ID: "s1b", ProjectID: "p1", StartedAt: now.Add(time.Second)},
		{ID: "s2a", ProjectID: "p2", StartedAt: now},
		{ID: "s2b", ProjectID: "p2", StartedAt: now.Add(time.Second)},
	}
	m.expanded = map[string]bool{"p1": true, "p2": true}
	// Start on p2 header row.
	m.cursor.projectIdx = 1
	m.cursor.agentIdx = -1

	type pos struct{ proj, agent int }
	steps := []struct {
		dir  string
		want pos
	}{
		{"up", pos{0, 1}},  // p1 last session
		{"up", pos{0, 0}},  // p1 first session
		{"up", pos{0, -1}}, // p1 header
		{"up", pos{0, -1}}, // no movement — already at top
		// reverse: navigate back down
		{"down", pos{0, 0}},  // p1 first session
		{"down", pos{0, 1}},  // p1 last session
		{"down", pos{1, -1}}, // p2 header
		{"down", pos{1, 0}},  // p2 first session
		{"down", pos{1, 1}},  // p2 last session
		{"down", pos{1, 1}},  // no movement — already at bottom
	}

	for i, s := range steps {
		var cmd tea.Cmd
		if s.dir == "up" {
			m, cmd = m.cursorUp()
		} else {
			m, cmd = m.cursorDown()
		}
		_ = cmd
		if m.cursor.projectIdx != s.want.proj || m.cursor.agentIdx != s.want.agent {
			t.Errorf("step %d (%s): want {proj=%d agent=%d}, got {proj=%d agent=%d}",
				i+1, s.dir, s.want.proj, s.want.agent, m.cursor.projectIdx, m.cursor.agentIdx)
		}
	}
}
