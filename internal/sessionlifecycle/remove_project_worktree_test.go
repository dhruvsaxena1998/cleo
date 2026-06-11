package sessionlifecycle_test

import (
	"errors"
	"testing"

	"github.com/dhruvsaxena1998/cleo/internal/sessionlifecycle"
)

func TestRemoveProjectSessionsAbortsUpfrontOnAnyDirtyWorktree(t *testing.T) {
	h := newWorktreeHarness(t)
	clean := killedWorktreeSession(t, h, "clean")
	dirty := killedWorktreeSession(t, h, "dirty")
	h.Worktree.dirty = map[string]bool{dirty.WorktreePath: true}

	result, err := h.Lifecycle.RemoveProjectSessions(sessionlifecycle.RemoveProjectSessionsInput{
		ProjectID: h.ProjectID,
	})
	if !errors.Is(err, sessionlifecycle.ErrDirtyWorktreesBlock) {
		t.Fatalf("error = %v, want ErrDirtyWorktreesBlock", err)
	}
	if len(result.DirtyWorktreeSessionIDs) != 1 || result.DirtyWorktreeSessionIDs[0] != dirty.ID {
		t.Fatalf("offenders = %v, want [%q]", result.DirtyWorktreeSessionIDs, dirty.ID)
	}

	// All-or-nothing: the clean session must be untouched too.
	if len(h.Worktree.removed) != 0 {
		t.Fatalf("aborted removal removed worktrees: %#v", h.Worktree.removed)
	}
	for _, id := range []string{clean.ID, dirty.ID} {
		if _, err := h.State.Get(id); err != nil {
			t.Fatalf("aborted removal deleted record %s: %v", id, err)
		}
	}
}

func TestRemoveProjectSessionsForceRemovesDirtyWorktrees(t *testing.T) {
	h := newWorktreeHarness(t)
	dirty := killedWorktreeSession(t, h, "dirty")
	h.Worktree.dirty = map[string]bool{dirty.WorktreePath: true}

	result, err := h.Lifecycle.RemoveProjectSessions(sessionlifecycle.RemoveProjectSessionsInput{
		ProjectID: h.ProjectID,
		Force:     true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.RemovedSessionIDs) != 1 {
		t.Fatalf("removed = %v", result.RemovedSessionIDs)
	}
	if len(h.Worktree.removed) != 1 || !h.Worktree.removed[0].Force {
		t.Fatalf("removals = %#v, want one forced", h.Worktree.removed)
	}
	if h.Worktree.removed[0].DeleteBranch != "" {
		t.Fatal("project removal never deletes branches")
	}
}

func TestRemoveProjectSessionsRemovesCleanWorktreesWithRecords(t *testing.T) {
	h := newWorktreeHarness(t)
	sess := killedWorktreeSession(t, h, "clean")

	result, err := h.Lifecycle.RemoveProjectSessions(sessionlifecycle.RemoveProjectSessionsInput{
		ProjectID: h.ProjectID,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.RemovedSessionIDs) != 1 || result.RemovedSessionIDs[0] != sess.ID {
		t.Fatalf("removed = %v", result.RemovedSessionIDs)
	}
	if len(h.Worktree.removed) != 1 || h.Worktree.removed[0].Dir != sess.WorktreePath {
		t.Fatalf("removals = %#v, want the session's worktree", h.Worktree.removed)
	}
	if _, err := h.State.Get(sess.ID); err == nil {
		t.Fatal("record should be gone")
	}
}
