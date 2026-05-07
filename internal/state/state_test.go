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

func TestStoreConcurrentApply(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(filepath.Join(dir, "state.json"), filepath.Join(dir, "state.json.lock"))
	_ = store.Put(Session{ID: "x", State: Running, Agent: "claude"})

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = store.Apply("x", EvPostToolUse, "")
		}()
	}
	wg.Wait()
	got, _ := store.Get("x")
	if got.ToolCount != 50 {
		t.Errorf("expected tool_count 50 (no lost updates), got %d", got.ToolCount)
	}
}
