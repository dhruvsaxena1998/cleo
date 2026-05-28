package reconcile

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/dhruvsaxena1998/cleo/internal/state"
)

// -- Pure Decide tests (no I/O, no temp dirs) --

func needAction(t *testing.T, actions []Action, wantSID string, wantEvent state.Event, wantBump bool) Action {
	t.Helper()
	for _, a := range actions {
		if a.SessionID == wantSID && a.Event == wantEvent {
			if a.BumpTime != wantBump {
				t.Errorf("session %s: BumpTime = %v, want %v", wantSID, a.BumpTime, wantBump)
			}
			return a
		}
	}
	t.Errorf("session %s: no action with event %s found among %d actions", wantSID, wantEvent, len(actions))
	return Action{}
}

func wantNoAction(t *testing.T, actions []Action, sid string) {
	t.Helper()
	for _, a := range actions {
		if a.SessionID == sid {
			t.Errorf("session %s: unexpected action %s", sid, a.Event)
			return
		}
	}
}

func TestDecide_MissingSessionMarkedDead(t *testing.T) {
	now := time.Now()
	sessions := []state.Session{
		{ID: "s1", State: state.Running, LastEventAt: now},
	}
	liveSet := map[string]bool{}

	actions := Decide(sessions, liveSet, now, Options{IdleTimeout: time.Hour, SpawningTimeout: 30 * time.Second})

	if len(actions) != 1 {
		t.Fatalf("want 1 action, got %d", len(actions))
	}
	a := actions[0]
	if a.SessionID != "s1" {
		t.Errorf("want s1, got %s", a.SessionID)
	}
	if a.Event != state.EvDead {
		t.Errorf("want EvDead, got %s", a.Event)
	}
	if a.BumpTime {
		t.Error("EvDead should not bump time")
	}
}

func TestDecide_ExistingDeadSessionNotReDead(t *testing.T) {
	now := time.Now()
	sessions := []state.Session{
		{ID: "s1", State: state.Dead, LastEventAt: now},
	}
	liveSet := map[string]bool{}

	actions := Decide(sessions, liveSet, now, Options{IdleTimeout: time.Hour, SpawningTimeout: 30 * time.Second})

	if len(actions) != 0 {
		t.Errorf("want 0 actions, got %d: %+v", len(actions), actions)
	}
}

func TestDecide_CompletedSessionWithLiveTmuxRevived(t *testing.T) {
	now := time.Now()
	sessions := []state.Session{
		{ID: "s1", State: state.Completed, LastEventAt: now.Add(-30 * time.Minute)},
	}
	liveSet := map[string]bool{"s1": true}

	actions := Decide(sessions, liveSet, now, Options{IdleTimeout: time.Hour, SpawningTimeout: 30 * time.Second})

	needAction(t, actions, "s1", state.EvUserResume, true)
}

func TestDecide_CompletedSessionWithDeadTmuxMarkedDead(t *testing.T) {
	now := time.Now()
	sessions := []state.Session{
		{ID: "s1", State: state.Completed, LastEventAt: now.Add(-30 * time.Minute)},
	}
	liveSet := map[string]bool{}

	actions := Decide(sessions, liveSet, now, Options{IdleTimeout: time.Hour, SpawningTimeout: 30 * time.Second})

	// Completed + not in live set → hits the first condition (missing not dead) → EvDead
	needAction(t, actions, "s1", state.EvDead, false)
}

func TestDecide_SpawningTimeoutAdvancesToRunning(t *testing.T) {
	now := time.Now()
	sessions := []state.Session{
		{ID: "s1", State: state.Spawning, StartedAt: now.Add(-1 * time.Minute)},
	}
	liveSet := map[string]bool{"s1": true}

	actions := Decide(sessions, liveSet, now, Options{IdleTimeout: time.Hour, SpawningTimeout: 5 * time.Second})

	a := needAction(t, actions, "s1", state.EvSessionStart, true)
	if a.Message == "" {
		t.Error("spawning advance action should carry a message")
	}
}

func TestDecide_SpawningTimeoutDoesNotFireEarly(t *testing.T) {
	now := time.Now()
	sessions := []state.Session{
		{ID: "s1", State: state.Spawning, StartedAt: now.Add(-1 * time.Second)},
	}
	liveSet := map[string]bool{"s1": true}

	actions := Decide(sessions, liveSet, now, Options{IdleTimeout: time.Hour, SpawningTimeout: 5 * time.Second})

	wantNoAction(t, actions, "s1")
}

func TestDecide_IdleTimeoutPromotesToCompleted(t *testing.T) {
	now := time.Now()
	sessions := []state.Session{
		{ID: "s1", State: state.Idle, LastEventAt: now.Add(-30 * time.Minute)},
	}
	liveSet := map[string]bool{"s1": true}

	actions := Decide(sessions, liveSet, now, Options{IdleTimeout: 10 * time.Minute, SpawningTimeout: 30 * time.Second})

	needAction(t, actions, "s1", state.EvIdleTimeout, false)
}

func TestDecide_WaitingForInputDowngradesToIdle(t *testing.T) {
	now := time.Now()
	sessions := []state.Session{
		{ID: "s1", State: state.WaitingForInput, LastEventAt: now.Add(-10 * time.Minute)},
	}
	liveSet := map[string]bool{"s1": true}

	actions := Decide(sessions, liveSet, now, Options{IdleTimeout: 1 * time.Minute, SpawningTimeout: 30 * time.Second})

	needAction(t, actions, "s1", state.EvIdleTimeout, false)
}

// Two-cycle test: WaitingForInput → Idle (first call), then Idle → Completed (second call).
// Simulates applying the action manually since Decide is pure.
func TestDecide_WaitingForInputTwoCycle(t *testing.T) {
	now := time.Now()
	baseSession := state.Session{ID: "s1", State: state.WaitingForInput, LastEventAt: now.Add(-10 * time.Minute)}
	liveSet := map[string]bool{"s1": true}
	opts := Options{IdleTimeout: 1 * time.Minute, SpawningTimeout: 30 * time.Second}

	// First cycle: WaitingForInput → Idle (EvIdleTimeout, no bump)
	actions1 := Decide([]state.Session{baseSession}, liveSet, now, opts)
	_ = needAction(t, actions1, "s1", state.EvIdleTimeout, false)

	// Simulate applying the action: state transitions to Idle, LastEventAt unchanged
	s1 := baseSession
	s1.State = state.NextState(s1.State, state.EvIdleTimeout)
	if s1.State != state.Idle {
		t.Fatalf("after first EvIdleTimeout, want Idle, got %s", s1.State)
	}
	// LastEventAt must NOT be bumped
	if !s1.LastEventAt.Equal(baseSession.LastEventAt) {
		t.Fatal("LastEventAt was bumped after EvIdleTimeout")
	}

	// Second cycle: Idle → Completed (EvIdleTimeout, no bump)
	actions2 := Decide([]state.Session{s1}, liveSet, now, opts)
	a := needAction(t, actions2, "s1", state.EvIdleTimeout, false)

	// Verify the transition would complete it
	completedState := state.NextState(s1.State, a.Event)
	if completedState != state.Completed {
		t.Fatalf("after second EvIdleTimeout, want Completed, got %s", completedState)
	}
}

func TestDecide_NoActionForRunningSession(t *testing.T) {
	now := time.Now()
	sessions := []state.Session{
		{ID: "s1", State: state.Running, LastEventAt: now},
	}
	liveSet := map[string]bool{"s1": true}

	actions := Decide(sessions, liveSet, now, Options{IdleTimeout: time.Hour, SpawningTimeout: 30 * time.Second})

	if len(actions) != 0 {
		t.Errorf("want 0 actions, got %d: %+v", len(actions), actions)
	}
}

func TestDecide_MultipleSessionsMixed(t *testing.T) {
	now := time.Now()
	sessions := []state.Session{
		{ID: "s1", State: state.Running, LastEventAt: now},                                // missing → EvDead
		{ID: "s2", State: state.Dead, LastEventAt: now},                                   // already dead → skip
		{ID: "s3", State: state.Completed, LastEventAt: now.Add(-30 * time.Minute)},        // live → EvUserResume
		{ID: "s4", State: state.Idle, LastEventAt: now.Add(-30 * time.Minute)},             // idle timeout → EvIdleTimeout
		{ID: "s5", State: state.Running, LastEventAt: now},                                // live running → skip
	}
	liveSet := map[string]bool{"s3": true, "s4": true, "s5": true}

	actions := Decide(sessions, liveSet, now, Options{IdleTimeout: 10 * time.Minute, SpawningTimeout: 30 * time.Second})

	if len(actions) != 3 {
		t.Fatalf("want 3 actions, got %d: %+v", len(actions), actions)
	}
	needAction(t, actions, "s1", state.EvDead, false)
	wantNoAction(t, actions, "s2")
	needAction(t, actions, "s3", state.EvUserResume, true)
	needAction(t, actions, "s4", state.EvIdleTimeout, false)
	wantNoAction(t, actions, "s5")
}

// -- Existing RunOpts integration tests (regression for the wrapper) --

type fakeTmux struct{ existing []string }

func (f *fakeTmux) LsPrefix(string) ([]string, error) { return f.existing, nil }

func TestReconcileMarksMissingSessionsDead(t *testing.T) {
	dir := t.TempDir()
	st := state.NewStore(filepath.Join(dir, "state.json"), filepath.Join(dir, "lock"))
	_ = st.Put(state.Session{ID: "cleo-foo-claude-1", State: state.Running, LastEventAt: time.Now()})
	_ = st.Put(state.Session{ID: "cleo-bar-claude-1", State: state.Running, LastEventAt: time.Now()})

	tx := &fakeTmux{existing: []string{"cleo-foo-claude-1"}}
	if err := RunOpts(st, tx, Options{IdleTimeout: time.Hour, SpawningTimeout: 30 * time.Second}); err != nil {
		t.Fatal(err)
	}
	got, _ := st.Get("cleo-bar-claude-1")
	if got.State != state.Dead {
		t.Errorf("expected dead, got %s", got.State)
	}
	got, _ = st.Get("cleo-foo-claude-1")
	if got.State != state.Running {
		t.Errorf("expected still running, got %s", got.State)
	}
}

func TestReconcileIdleTimeoutPromotesToCompleted(t *testing.T) {
	dir := t.TempDir()
	st := state.NewStore(filepath.Join(dir, "state.json"), filepath.Join(dir, "lock"))
	_ = st.Put(state.Session{
		ID: "cleo-foo-claude-1", State: state.Idle, LastEventAt: time.Now().Add(-30 * time.Minute),
	})
	tx := &fakeTmux{existing: []string{"cleo-foo-claude-1"}}
	if err := RunOpts(st, tx, Options{IdleTimeout: 10 * time.Minute, SpawningTimeout: 30 * time.Second}); err != nil {
		t.Fatal(err)
	}
	got, _ := st.Get("cleo-foo-claude-1")
	if got.State != state.Completed {
		t.Errorf("expected completed, got %s", got.State)
	}
}

// Regression: synthetic EvIdleTimeout used to bump LastEventAt and trap
// WaitingForInput sessions in an indefinite loop because each reconcile
// cycle reset the idle clock. The two-cycle assertion below pins the bug
// shut — first cycle must not bump LastEventAt, second cycle must reach
// Completed using the same anchor timestamp.
func TestWaitingForInputProgressesToCompletedAcrossTwoIdleCycles(t *testing.T) {
	dir := t.TempDir()
	st := state.NewStore(filepath.Join(dir, "state.json"), filepath.Join(dir, "state.json.lock"))
	tx := &fakeTmux{existing: []string{"s1"}}

	tenMinAgo := time.Now().Add(-10 * time.Minute)
	if err := st.Put(state.Session{ID: "s1", State: state.WaitingForInput, LastEventAt: tenMinAgo}); err != nil {
		t.Fatalf("put: %v", err)
	}

	// First reconcile: WaitingForInput -> Idle. LastEventAt must NOT be bumped.
	if err := RunOpts(st, tx, Options{IdleTimeout: time.Minute, SpawningTimeout: 30 * time.Second}); err != nil {
		t.Fatalf("first reconcile: %v", err)
	}
	got, _ := st.Get("s1")
	if got.State != state.Idle {
		t.Fatalf("after first reconcile, want Idle, got %s", got.State)
	}
	if !got.LastEventAt.Equal(tenMinAgo) {
		t.Fatalf("LastEventAt bumped: want %v, got %v", tenMinAgo, got.LastEventAt)
	}

	// Second reconcile (immediate): Idle -> Completed because LastEventAt is still 10min ago.
	if err := RunOpts(st, tx, Options{IdleTimeout: time.Minute, SpawningTimeout: 30 * time.Second}); err != nil {
		t.Fatalf("second reconcile: %v", err)
	}
	got, _ = st.Get("s1")
	if got.State != state.Completed {
		t.Fatalf("after second reconcile, want Completed, got %s", got.State)
	}
}

func TestSpawningTimeoutAdvanceSetsLastMessage(t *testing.T) {
	dir := t.TempDir()
	st := state.NewStore(filepath.Join(dir, "state.json"), filepath.Join(dir, "state.json.lock"))
	tx := &fakeTmux{existing: []string{"s1"}}

	if err := st.Put(state.Session{
		ID: "s1", State: state.Spawning,
		StartedAt: time.Now().Add(-time.Minute),
	}); err != nil {
		t.Fatalf("put: %v", err)
	}

	if err := RunOpts(st, tx, Options{IdleTimeout: 10 * time.Minute, SpawningTimeout: 5 * time.Second}); err != nil {
		t.Fatalf("reconcile: %v", err)
	}
	got, _ := st.Get("s1")
	if got.State != state.Running {
		t.Fatalf("want Running, got %s", got.State)
	}
	if !strings.Contains(got.LastMessage, "spawning") {
		t.Fatalf("LastMessage should mention spawning, got %q", got.LastMessage)
	}
}

func TestReconcileRevivesCompletedSessionWithLiveTmux(t *testing.T) {
	dir := t.TempDir()
	st := state.NewStore(filepath.Join(dir, "state.json"), filepath.Join(dir, "state.json.lock"))
	tx := &fakeTmux{existing: []string{"s1"}}

	oldTime := time.Now().Add(-30 * time.Minute)
	if err := st.Put(state.Session{
		ID: "s1", State: state.Completed, LastEventAt: oldTime,
	}); err != nil {
		t.Fatalf("put: %v", err)
	}

	if err := RunOpts(st, tx, Options{IdleTimeout: 10 * time.Minute, SpawningTimeout: 30 * time.Second}); err != nil {
		t.Fatalf("reconcile: %v", err)
	}
	got, _ := st.Get("s1")
	if got.State != state.Idle {
		t.Fatalf("want Idle (revived), got %s", got.State)
	}
	// LastEventAt must be bumped so the idle clock restarts (no immediate re-timeout).
	if !got.LastEventAt.After(oldTime) {
		t.Fatalf("LastEventAt not bumped: want after %v, got %v", oldTime, got.LastEventAt)
	}
}

func TestReconcileDoesNotReviveCompletedWithDeadTmux(t *testing.T) {
	dir := t.TempDir()
	st := state.NewStore(filepath.Join(dir, "state.json"), filepath.Join(dir, "state.json.lock"))
	tx := &fakeTmux{existing: []string{}} // tmux reports session gone

	if err := st.Put(state.Session{
		ID: "s1", State: state.Completed, LastEventAt: time.Now().Add(-30 * time.Minute),
	}); err != nil {
		t.Fatalf("put: %v", err)
	}

	if err := RunOpts(st, tx, Options{IdleTimeout: 10 * time.Minute, SpawningTimeout: 30 * time.Second}); err != nil {
		t.Fatalf("reconcile: %v", err)
	}
	got, _ := st.Get("s1")
	if got.State != state.Dead {
		t.Fatalf("want Dead (tmux gone), got %s", got.State)
	}
}

func TestRunOptsUsesProvidedSpawningTimeout(t *testing.T) {
	dir := t.TempDir()
	st := state.NewStore(filepath.Join(dir, "state.json"), filepath.Join(dir, "state.json.lock"))
	tx := &fakeTmux{existing: []string{"s1"}}

	// Started 5s ago; SpawningTimeout = 1s should fire, default 30s should not.
	if err := st.Put(state.Session{
		ID: "s1", State: state.Spawning,
		StartedAt: time.Now().Add(-5 * time.Second),
	}); err != nil {
		t.Fatalf("put: %v", err)
	}

	if err := RunOpts(st, tx, Options{SpawningTimeout: time.Second, IdleTimeout: time.Hour}); err != nil {
		t.Fatalf("reconcile: %v", err)
	}
	got, _ := st.Get("s1")
	if got.State != state.Running {
		t.Fatalf("with 1s timeout and 5s elapsed, want Running, got %s", got.State)
	}
}
