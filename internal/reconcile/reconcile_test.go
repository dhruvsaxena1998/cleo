package reconcile

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/dhruvsaxena1998/cleo/internal/state"
)

type fakeTmux struct{ existing []string }

func (f *fakeTmux) LsPrefix(string) ([]string, error) { return f.existing, nil }

func TestReconcileMarksMissingSessionsDead(t *testing.T) {
	dir := t.TempDir()
	st := state.NewStore(filepath.Join(dir, "state.json"), filepath.Join(dir, "lock"))
	_ = st.Put(state.Session{ID: "cleo-foo-claude-1", State: state.Running, LastEventAt: time.Now()})
	_ = st.Put(state.Session{ID: "cleo-bar-claude-1", State: state.Running, LastEventAt: time.Now()})

	tx := &fakeTmux{existing: []string{"cleo-foo-claude-1"}}
	if err := Run(st, tx, time.Hour); err != nil {
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
	if err := Run(st, tx, 10*time.Minute); err != nil {
		t.Fatal(err)
	}
	got, _ := st.Get("cleo-foo-claude-1")
	if got.State != state.Completed {
		t.Errorf("expected completed, got %s", got.State)
	}
}

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
