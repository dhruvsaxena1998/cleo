package sessionlifecycle_test

import (
	"errors"
	"testing"

	"github.com/dhruvsaxena1998/cleo/internal/sessionlifecycle"
	"github.com/dhruvsaxena1998/cleo/internal/state"
)

// killedWorktreeSession spawns a worktree session through the public API and
// kills it, returning the dead-but-recorded session — the normal state rm
// operates on.
func killedWorktreeSession(t *testing.T, h *worktreeHarness, name string) state.Session {
	t.Helper()
	result, err := h.Lifecycle.Create(sessionlifecycle.CreateInput{
		Agent:     "claude",
		Name:      name,
		ProjectID: h.ProjectID,
		Worktree:  boolPtr(true),
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := h.Lifecycle.Kill(result.Session.ID); err != nil {
		t.Fatal(err)
	}
	return result.Session
}

func TestRemoveSessionSkipsDirtyWorktreeKeepingRecord(t *testing.T) {
	h := newWorktreeHarness(t)
	sess := killedWorktreeSession(t, h, "dirty-work")
	h.Worktree.dirty = map[string]bool{sess.WorktreePath: true}

	_, err := h.Lifecycle.RemoveSession(sessionlifecycle.RemoveSessionInput{
		SessionID: sess.ID,
	})
	if !errors.Is(err, sessionlifecycle.ErrWorktreeDirty) {
		t.Fatalf("RemoveSession error = %v, want ErrWorktreeDirty", err)
	}

	for _, call := range h.Worktree.calls {
		if call == "remove" {
			t.Fatal("dirty worktree must not be removed without force")
		}
	}
	if _, err := h.State.Get(sess.ID); err != nil {
		t.Fatalf("dirty skip must keep the record too (live and die together), got err=%v", err)
	}
}

func TestRemoveSessionForceRemovesDirtyWorktree(t *testing.T) {
	h := newWorktreeHarness(t)
	sess := killedWorktreeSession(t, h, "abandoned-work")
	h.Worktree.dirty = map[string]bool{sess.WorktreePath: true}

	result, err := h.Lifecycle.RemoveSession(sessionlifecycle.RemoveSessionInput{
		SessionID: sess.ID,
		Force:     true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !result.Removed {
		t.Fatalf("result = %#v, want Removed", result)
	}
	if len(h.Worktree.removed) != 1 || !h.Worktree.removed[0].Force {
		t.Fatalf("worktree removals = %#v, want one forced", h.Worktree.removed)
	}
	if h.Worktree.removed[0].DeleteBranch != "" {
		t.Fatal("even forced rm never deletes the branch")
	}
	if _, err := h.State.Get(sess.ID); err == nil {
		t.Fatal("record should be gone after forced rm")
	}
}

func TestRemoveSessionRefusesActiveSessionWithoutForce(t *testing.T) {
	h := newWorktreeHarness(t)
	result, err := h.Lifecycle.Create(sessionlifecycle.CreateInput{
		Agent:     "claude",
		Name:      "still-running",
		ProjectID: h.ProjectID,
		Worktree:  boolPtr(true),
	})
	if err != nil {
		t.Fatal(err)
	}

	_, err = h.Lifecycle.RemoveSession(sessionlifecycle.RemoveSessionInput{
		SessionID: result.Session.ID,
	})
	if !errors.Is(err, sessionlifecycle.ErrSessionActive) {
		t.Fatalf("RemoveSession error = %v, want ErrSessionActive", err)
	}
	if _, err := h.State.Get(result.Session.ID); err != nil {
		t.Fatalf("active session must be untouched, got err=%v", err)
	}
	if len(h.Worktree.removed) != 0 {
		t.Fatalf("active session's worktree must be untouched: %#v", h.Worktree.removed)
	}
}

func TestRemoveSessionForceKillsActiveSessionThenRemoves(t *testing.T) {
	h := newWorktreeHarness(t)
	result, err := h.Lifecycle.Create(sessionlifecycle.CreateInput{
		Agent:     "claude",
		Name:      "kill-and-remove",
		ProjectID: h.ProjectID,
		Worktree:  boolPtr(true),
	})
	if err != nil {
		t.Fatal(err)
	}

	removeResult, err := h.Lifecycle.RemoveSession(sessionlifecycle.RemoveSessionInput{
		SessionID: result.Session.ID,
		Force:     true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !removeResult.Removed {
		t.Fatalf("result = %#v, want Removed", removeResult)
	}
	if len(h.Tmux.killed) != 1 || h.Tmux.killed[0] != result.Session.ID {
		t.Fatalf("tmux killed = %v, want the forced session", h.Tmux.killed)
	}
	if _, err := h.State.Get(result.Session.ID); err == nil {
		t.Fatal("record should be gone after forced rm")
	}
}

func TestRemoveSessionRemovesPlainSessionRecordWithoutTouchingSeam(t *testing.T) {
	h := newWorktreeHarness(t)
	result, err := h.Lifecycle.Create(sessionlifecycle.CreateInput{
		Agent:     "claude",
		Name:      "plain-done",
		ProjectID: h.ProjectID,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := h.Lifecycle.Kill(result.Session.ID); err != nil {
		t.Fatal(err)
	}
	// Plain sessions are deleted by kill; reseed a finished record the way the
	// reconciler would leave one.
	if err := h.State.Put(state.Session{
		ID: result.Session.ID, ProjectID: h.ProjectID, Agent: "claude",
		Name: "plain-done", State: state.Completed,
	}); err != nil {
		t.Fatal(err)
	}
	h.Worktree.calls = nil

	if _, err := h.Lifecycle.RemoveSession(sessionlifecycle.RemoveSessionInput{
		SessionID: result.Session.ID,
	}); err != nil {
		t.Fatal(err)
	}
	if len(h.Worktree.calls) != 0 {
		t.Fatalf("plain rm touched the worktree seam: %v", h.Worktree.calls)
	}
	if _, err := h.State.Get(result.Session.ID); err == nil {
		t.Fatal("plain record should be gone after rm")
	}
}

func TestRemoveSessionRemovesCleanWorktreeWithRecord(t *testing.T) {
	h := newWorktreeHarness(t)
	sess := killedWorktreeSession(t, h, "done-work")

	result, err := h.Lifecycle.RemoveSession(sessionlifecycle.RemoveSessionInput{
		SessionID: sess.ID,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !result.Removed {
		t.Fatalf("result = %#v, want Removed", result)
	}

	if len(h.Worktree.removed) != 1 {
		t.Fatalf("worktree removals = %#v, want one", h.Worktree.removed)
	}
	got := h.Worktree.removed[0]
	if got.Dir != sess.WorktreePath {
		t.Fatalf("removed dir = %q, want %q", got.Dir, sess.WorktreePath)
	}
	if got.ProjectPath != h.ProjectPath {
		t.Fatalf("removed ProjectPath = %q, want %q", got.ProjectPath, h.ProjectPath)
	}
	if got.Force {
		t.Fatal("unforced rm must not force-remove the worktree")
	}
	if got.DeleteBranch != "" {
		t.Fatalf("rm deleted branch %q; committed work must stay reachable", got.DeleteBranch)
	}

	if _, err := h.State.Get(sess.ID); err == nil {
		t.Fatal("session record should be gone after rm")
	}
}
