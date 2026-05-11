package state

import (
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func TestNextState(t *testing.T) {
	cases := []struct {
		from State
		ev   Event
		want State
	}{
		{Spawning, EvSessionStart, Running},
		{Spawning, EvPreToolUse, Running},
		{Running, EvPreToolUse, Running}, // no-op
		{Running, EvPostToolUse, Running},
		{Running, EvNotification, WaitingForInput},
		{WaitingForInput, EvUserResume, Running},
		{Running, EvStop, Idle},
		{Idle, EvSessionEnd, Completed},
		{Idle, EvIdleTimeout, Completed},
		{Running, EvSessionEnd, Completed},
		{Idle, EvError, Errored},
		// Terminal states must not be resurrected by hook events.
		{Dead, EvNotification, Dead},
		{Dead, EvStop, Dead},
		{Dead, EvSessionStart, Dead},
		{Dead, EvUserResume, Dead},
		{Dead, EvPreToolUse, Dead},
		{Dead, EvPostToolUse, Dead},
		{Dead, EvSessionEnd, Dead},
		{Completed, EvNotification, Completed},
		{Completed, EvStop, Completed},
		{Completed, EvSessionStart, Completed},
		{Errored, EvNotification, Errored},
		{Errored, EvStop, Errored},
		// EvDead is still allowed — idempotent and absorbing.
		{Dead, EvDead, Dead},
		{Completed, EvDead, Dead},
		{Errored, EvDead, Dead},
	}
	for _, c := range cases {
		got := NextState(c.from, c.ev)
		if got != c.want {
			t.Errorf("NextState(%s, %s) = %s, want %s", c.from, c.ev, got, c.want)
		}
	}
}

func TestStorePutGet(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(filepath.Join(dir, "state.json"), filepath.Join(dir, "state.json.lock"))

	s := Session{
		ID: "cleo-foo-claude-1", ProjectID: "foo", Agent: "claude",
		Name: "1", State: Spawning, StartedAt: time.Now().UTC(),
	}
	if err := store.Put(s); err != nil {
		t.Fatal(err)
	}
	got, err := store.Get(s.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.State != Spawning {
		t.Errorf("state %s", got.State)
	}
}

func TestStoreApplyEvent(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(filepath.Join(dir, "state.json"), filepath.Join(dir, "state.json.lock"))
	s := Session{ID: "x", State: Spawning, Agent: "claude"}
	_ = store.Put(s)

	got, err := store.Apply("x", EvSessionStart, "")
	if err != nil {
		t.Fatal(err)
	}
	if got.State != Running {
		t.Errorf("state %s", got.State)
	}
	if got.ToolCount != 0 {
		t.Errorf("tool count %d", got.ToolCount)
	}

	got, _ = store.Apply("x", EvPostToolUse, "")
	if got.ToolCount != 1 {
		t.Errorf("expected tool_count 1, got %d", got.ToolCount)
	}
}

func TestApplySyntheticDoesNotBumpLastEventAt(t *testing.T) {
	dir := t.TempDir()
	st := NewStore(filepath.Join(dir, "state.json"), filepath.Join(dir, "state.json.lock"))
	at := time.Date(2026, 5, 1, 12, 0, 0, 0, time.UTC)
	if err := st.Put(Session{ID: "s1", State: WaitingForInput, LastEventAt: at}); err != nil {
		t.Fatalf("put: %v", err)
	}

	out, err := st.ApplySynthetic("s1", EvIdleTimeout, "")
	if err != nil {
		t.Fatalf("apply synthetic: %v", err)
	}

	if out.State != Idle {
		t.Errorf("state: want Idle, got %s", out.State)
	}
	if !out.LastEventAt.Equal(at) {
		t.Errorf("LastEventAt was bumped: want %v, got %v", at, out.LastEventAt)
	}
}

func TestStoreConcurrentApply(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(filepath.Join(dir, "state.json"), filepath.Join(dir, "state.json.lock"))
	_ = store.Put(Session{ID: "x", State: Running, Agent: "claude"})

	var wg sync.WaitGroup
	for range 50 {
		wg.Go(func() {
			_, _ = store.Apply("x", EvPostToolUse, "")
		})
	}
	wg.Wait()
	got, _ := store.Get("x")
	if got.ToolCount != 50 {
		t.Errorf("expected tool_count 50 (no lost updates), got %d", got.ToolCount)
	}
}
