package tui

import (
	"os"
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
	c.Tmux = &fakeTmux{live: map[string]bool{}}
	return c
}

func mkdirAll(p string) error { return os.MkdirAll(p, 0o755) }

func contains(b []byte, s string) bool {
	return strings.Contains(string(b), s)
}

// fakeTmux is a test double for cli.TmuxClient that reports a fixed set of live sessions.
type fakeTmux struct {
	live map[string]bool
}

func (f *fakeTmux) NewSession(_ tmux.NewSessionOpts) error { return nil }
func (f *fakeTmux) HasSession(n string) (bool, error)      { return f.live[n], nil }
func (f *fakeTmux) LsPrefix(prefix string) ([]string, error) {
	var out []string
	for n := range f.live {
		if strings.HasPrefix(n, prefix) {
			out = append(out, n)
		}
	}
	return out, nil
}
func (f *fakeTmux) Kill(n string) error                     { delete(f.live, n); return nil }
func (f *fakeTmux) CapturePane(string, int) (string, error) { return "", nil }
func (f *fakeTmux) RenameSession(from, to string) error     { return nil }

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

// TestPreviewTickAlwaysReArms locks in the self-recovering preview ticker:
// every previewTickMsg fire must return a non-nil command that produces another
// previewTickMsg, regardless of whether a capture was dispatched. The previous
// chain (paneCapturedMsg -> tea.Tick -> capturePaneTickMsg) deadlocked when
// the user navigated mid-flight, because the tick was scheduled from the
// *response* path; if the response sid no longer matched the selection, the
// loop returned m, nil and never fired again until manual nav.
func TestPreviewTickAlwaysReArms(t *testing.T) {
	c := newTestCtx(t)
	c.Config.UI.PanePreviewInterval = 10 * time.Millisecond
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
