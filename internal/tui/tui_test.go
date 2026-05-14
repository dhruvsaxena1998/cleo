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
		return contains(b, "cl") && contains(b, "◉")
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
		return contains(b, "cl") && contains(b, "◉")
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

// updateAsModel runs Update and asserts the resulting tea.Model is a Model.
// Used by the Esc-hierarchy and status-clear tests below.
func updateAsModel(m Model, msg tea.Msg) Model {
	out, _ := m.Update(msg)
	return out.(Model)
}

// TestEscClosesPopupOnly locks in step 1 of the Esc hierarchy: when a popup
// is open, Esc closes the popup and clears the status line, but leaves the
// filter query intact so the user returns to the same filtered view.
func TestEscClosesPopupOnly(t *testing.T) {
	c := newTestCtx(t)
	m := New(c)
	m.popup = NewHelpPopup(m.theme, "")
	m.mode = ModePopup
	m.filter = "active-query"
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
	if m2.filter != "active-query" {
		t.Errorf("filter should survive popup close, got %q", m2.filter)
	}
}

// TestEscInFilterClearsQueryAndExits locks in step 2 of the Esc hierarchy:
// in filter mode, Esc exits ModeFilter, clears the filter query, AND calls
// clampCursor so the cursor lands on a valid row after the visible set
// changes.
func TestEscInFilterClearsQueryAndExits(t *testing.T) {
	c := newTestCtx(t)
	m := New(c)
	m.projects = []projects.Project{{ID: "p1"}}
	m.mode = ModeFilter
	m.filter = "search"
	// Seed an out-of-range cursor that clampCursor must repair. Without the
	// clamp call in the Esc handler, projectIdx=99 would survive and the
	// next render would crash on a slice access.
	m.cursor.projectIdx = 99

	m2 := updateAsModel(m, tea.KeyMsg{Type: tea.KeyEsc})
	if m2.mode != ModeNormal {
		t.Errorf("mode: want Normal, got %v", m2.mode)
	}
	if m2.filter != "" {
		t.Errorf("filter: want empty, got %q", m2.filter)
	}
	if m2.cursor.projectIdx != 0 {
		t.Errorf("cursor not clamped: want projectIdx=0, got %d", m2.cursor.projectIdx)
	}
}

// TestEscInNormalClearsStatus locks in step 3 of the Esc hierarchy: with no
// popup or filter active, Esc just clears any stale status line message.
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

// TestStatusClearsOnFilterEntry covers the v0.2 status auto-clear extension:
// pressing '/' to enter filter mode must wipe a stale status line, just like
// cursor moves and expand/collapse already do.
func TestStatusClearsOnFilterEntry(t *testing.T) {
	c := newTestCtx(t)
	m := New(c)
	m.status = "old"

	m2 := updateAsModel(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
	if m2.mode != ModeFilter {
		t.Fatalf("'/' should enter filter mode, got mode=%v", m2.mode)
	}
	if m2.status != "" {
		t.Errorf("status should clear on filter entry, got %q", m2.status)
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

// TestFilterSurvivesExpandCollapse locks in spec §2.2's filter persistence
// guarantee: toggling a project's expand/collapse state must not clear the
// filter query, even though the visible row set re-flows.
func TestFilterSurvivesExpandCollapse(t *testing.T) {
	c := newTestCtx(t)
	m := New(c)
	m.projects = []projects.Project{{ID: "p1"}, {ID: "p2"}}
	m.sessions = []state.Session{{ID: "s1", ProjectID: "p1", Name: "alpha"}}
	m.filter = "alpha"
	m.expanded = map[string]bool{"p1": false}
	m.cursor.projectIdx = 0

	m2, _ := m.toggleExpand()
	if m2.filter != "alpha" {
		t.Errorf("filter cleared on expand: got %q", m2.filter)
	}
	// Sanity-check that toggleExpand actually flipped the project state, so
	// a future regression that no-ops the call doesn't pass this test by
	// accident.
	if !m2.expanded["p1"] {
		t.Errorf("expected p1 expanded after toggle, got %v", m2.expanded)
	}

	m3, _ := m2.toggleExpand()
	if m3.filter != "alpha" {
		t.Errorf("filter cleared on collapse: got %q", m3.filter)
	}
	if m3.expanded["p1"] {
		t.Errorf("expected p1 collapsed after second toggle, got %v", m3.expanded)
	}
}

func TestToDisplayState(t *testing.T) {
	cases := []struct {
		in   state.State
		want DisplayState
	}{
		{state.WaitingForInput, DisplayNeedsInput},
		{state.Running, DisplayWorking},
		{state.Spawning, DisplayWorking},
		{state.Idle, DisplayIdle},
		{state.Completed, DisplayCompleted},
		{state.Errored, DisplayFailed},
		{state.Dead, DisplayStopped},
	}
	for _, c := range cases {
		got := ToDisplayState(c.in)
		if got != c.want {
			t.Errorf("ToDisplayState(%s): want %d, got %d", c.in, c.want, got)
		}
	}
}

func TestUrgencyOrder(t *testing.T) {
	order := []DisplayState{DisplayNeedsInput, DisplayWorking, DisplayIdle, DisplayCompleted, DisplayFailed, DisplayStopped}
	for i := 1; i < len(order); i++ {
		if urgencyOrder(order[i-1]) >= urgencyOrder(order[i]) {
			t.Errorf("urgency: %d (%d) should be < %d (%d)", order[i-1], urgencyOrder(order[i-1]), order[i], urgencyOrder(order[i]))
		}
	}
}

func TestDisplayStateGlyph(t *testing.T) {
	want := map[DisplayState]string{
		DisplayNeedsInput: "⚠",
		DisplayWorking:    "◉",
		DisplayIdle:       "∙",
		DisplayCompleted:  "✓",
		DisplayFailed:     "✗",
		DisplayStopped:    "○",
	}
	for ds, g := range want {
		if got := displayStateGlyph(ds); got != g {
			t.Errorf("displayStateGlyph(%d): want %q, got %q", ds, g, got)
		}
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

func TestSessionsForSortsByUrgency(t *testing.T) {
	c := newTestCtx(t)
	m := New(c)
	m.projects = []projects.Project{{ID: "p1"}}
	now := time.Now()
	m.sessions = []state.Session{
		{ID: "s-idle", ProjectID: "p1", State: state.Idle, StartedAt: now},
		{ID: "s-run", ProjectID: "p1", State: state.Running, StartedAt: now},
		{ID: "s-wait", ProjectID: "p1", State: state.WaitingForInput, StartedAt: now},
		{ID: "s-done", ProjectID: "p1", State: state.Completed, StartedAt: now},
		{ID: "s-err", ProjectID: "p1", State: state.Errored, StartedAt: now},
		{ID: "s-dead", ProjectID: "p1", State: state.Dead, StartedAt: now},
	}
	got := m.sessionsFor("p1")
	wantOrder := []string{"s-wait", "s-run", "s-idle", "s-done", "s-err", "s-dead"}
	for i, want := range wantOrder {
		if i >= len(got) {
			t.Errorf("position %d: want %s, got <missing>", i, want)
			continue
		}
		if got[i].ID != want {
			t.Errorf("position %d: want %s, got %s", i, want, got[i].ID)
		}
	}
}

func TestAnimFrameIncrementsOnTick(t *testing.T) {
	c := newTestCtx(t)
	m := New(c)
	if m.animFrame != 0 {
		t.Fatalf("initial animFrame should be 0, got %d", m.animFrame)
	}
	m2, _ := m.Update(tickStateMsg{})
	m2model := m2.(Model)
	if m2model.animFrame != 1 {
		t.Fatalf("after first tick, animFrame should be 1, got %d", m2model.animFrame)
	}
	m3, _ := m2model.Update(tickStateMsg{})
	m3model := m3.(Model)
	if m3model.animFrame != 0 {
		t.Fatalf("after second tick, animFrame should cycle back to 0, got %d", m3model.animFrame)
	}
}

func TestEnterOnErroredSessionWithLiveTmuxAttaches(t *testing.T) {
	c := newTestCtx(t)
	c.Tmux.(*fakeTmux).live["cleo-myapp-claude-1"] = true
	m := New(c)
	m.projects = []projects.Project{{ID: "myapp"}}
	m.sessions = []state.Session{{
		ID: "cleo-myapp-claude-1", ProjectID: "myapp", Agent: "claude",
		State: state.Errored, StartedAt: time.Now(), LastEventAt: time.Now(),
	}}
	m.expanded["myapp"] = true
	m.cursor.projectIdx = 0
	m.cursor.agentIdx = 0

	got, _ := m.attachSelectedAgent()
	if strings.Contains(got.status, "press K") {
		t.Fatal("errored session with live tmux should not be blocked from attach")
	}
}

// TestHelpPopupView locks in the two-column help panel layout.
// It checks width, required sections, icons, and detach key substitution.
func TestHelpPopupView(t *testing.T) {
	theme := catppuccinMocha
	popup := NewHelpPopup(theme, "C-b d")
	out := popup.View()
	lines := strings.Split(out, "\n")

	// Every rendered line must fit within 91 terminal cells.
	const maxW = 91
	for i, line := range lines {
		if w := lipgloss.Width(line); w > maxW {
			t.Errorf("line %d too wide: %d > %d: %q", i, w, maxW, line)
		}
	}

	// All required section headers must appear.
	for _, want := range []string{
		"Navigation", "Session Actions", "Global", "tmux",
		"Icon Legend", "Filter", "Config",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing section %q in help output", want)
		}
	}

	// All six status icons must appear.
	for _, icon := range []string{"◉", "⚠", "✓", "✗", "∙", "○"} {
		if !strings.Contains(out, icon) {
			t.Errorf("missing icon %q in help output", icon)
		}
	}

	// Detach key should be formatted and present.
	if !strings.Contains(out, "ctrl+b d") {
		t.Errorf("detach key 'ctrl+b d' not found in help output")
	}

	// Config path must appear.
	if !strings.Contains(out, "~/.config/cleo/config.toml") {
		t.Errorf("config path not found in help output")
	}

	// Filter description must appear.
	if !strings.Contains(out, "project · session · agent") {
		t.Errorf("filter description not found in help output")
	}
}
