package reconcile

import (
	"path/filepath"
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
