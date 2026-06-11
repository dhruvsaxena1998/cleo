package state

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
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
		// Completed can be revived by EvUserResume (user re-attach or reconciler).
		{Completed, EvUserResume, Idle},
		// Dead and Errored resist EvUserResume (hard terminal).
		{Errored, EvUserResume, Errored},
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

func TestWorktreeFieldsRoundTrip(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(filepath.Join(dir, "state.json"), filepath.Join(dir, "state.json.lock"))

	s := Session{
		ID: "cleo-foo-claude-1", ProjectID: "foo", Agent: "claude",
		Name: "1", State: Spawning, StartedAt: time.Now().UTC(),
		WorktreePath:   "/Users/x/Dev/myapp/.cleo/worktrees/claude-1",
		WorktreeBranch: "cleo/wt-claude-1",
	}
	if err := store.Put(s); err != nil {
		t.Fatal(err)
	}
	got, err := store.Get(s.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.WorktreePath != s.WorktreePath {
		t.Errorf("worktree path %q", got.WorktreePath)
	}
	if got.WorktreeBranch != s.WorktreeBranch {
		t.Errorf("worktree branch %q", got.WorktreeBranch)
	}
}

func TestWorktreeFieldsOmittedForMainTreeSessions(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")
	store := NewStore(path, path+".lock")

	if err := store.Put(Session{ID: "s1", State: Running}); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(b), "worktree") {
		t.Errorf("main-tree session serialized worktree fields: %s", b)
	}
}

// A cleo binary must never destroy session fields written by a newer cleo: a
// long-running dashboard polls state.json every 750ms, so one stale binary in
// the mix silently wipes any field it doesn't know (exactly how the worktree
// badge vanished when an old binary's reconcile rewrote the record).
func TestReadModifyWritePreservesUnknownFieldsFromNewerSchemas(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")
	written := `{
		"version": 1,
		"sessions": {
			"s1": {
				"id": "s1",
				"state": "running",
				"tool_count": 2,
				"future_field": "from-a-newer-cleo",
				"future_obj": {"nested": true}
			}
		}
	}`
	if err := os.WriteFile(path, []byte(written), 0o644); err != nil {
		t.Fatal(err)
	}
	store := NewStore(path, path+".lock")

	// Read-modify-write through this (older) schema.
	if _, err := store.Apply("s1", EvStop, ""); err != nil {
		t.Fatal(err)
	}

	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"future_field", "from-a-newer-cleo", "future_obj"} {
		if !strings.Contains(string(b), want) {
			t.Fatalf("rewrite dropped unknown field %q:\n%s", want, b)
		}
	}
	got, err := store.Get("s1")
	if err != nil {
		t.Fatal(err)
	}
	if got.State != Idle {
		t.Fatalf("known-field update lost: state = %s, want idle", got.State)
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

func TestStoreUpdateNotFound(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(filepath.Join(dir, "state.json"), filepath.Join(dir, "state.json.lock"))

	called := false
	_, err := store.Update("missing", func(s *Session) error {
		called = true
		return nil
	})
	if !errors.Is(err, ErrSessionNotFound) {
		t.Errorf("want ErrSessionNotFound, got %v", err)
	}
	if called {
		t.Error("mutate should not run for a missing session")
	}
}

func TestStoreUpdateAbortsOnError(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(filepath.Join(dir, "state.json"), filepath.Join(dir, "state.json.lock"))
	_ = store.Put(Session{ID: "x", State: Running, Name: "orig"})

	sentinel := errors.New("reject")
	_, err := store.Update("x", func(s *Session) error {
		s.Name = "should-not-persist"
		return sentinel
	})
	if !errors.Is(err, sentinel) {
		t.Fatalf("want sentinel error, got %v", err)
	}
	got, _ := store.Get("x")
	if got.Name != "orig" {
		t.Errorf("aborted Update must not persist: name = %q", got.Name)
	}
}

// TestStoreUpdateApplyNoClobber is the regression test for the read-modify-write
// window that the old Get+mutate+Put Rename had: a rename (Update of Name)
// running concurrently with hook activity (Apply bumping ToolCount/State) must
// not clobber either writer. Both paths now serialize through the same lock, so
// every ToolCount bump survives and the rename takes effect.
func TestStoreUpdateApplyNoClobber(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(filepath.Join(dir, "state.json"), filepath.Join(dir, "state.json.lock"))
	_ = store.Put(Session{ID: "x", State: Running, Name: "orig"})

	var wg sync.WaitGroup
	for range 50 {
		wg.Go(func() { _, _ = store.Apply("x", EvPostToolUse, "") })
		wg.Go(func() {
			_, _ = store.Update("x", func(s *Session) error {
				s.Name = "renamed"
				return nil
			})
		})
	}
	wg.Wait()

	got, _ := store.Get("x")
	if got.ToolCount != 50 {
		t.Errorf("lost Apply updates under concurrent Update: tool_count = %d, want 50", got.ToolCount)
	}
	if got.Name != "renamed" {
		t.Errorf("rename clobbered: name = %q, want %q", got.Name, "renamed")
	}
}
