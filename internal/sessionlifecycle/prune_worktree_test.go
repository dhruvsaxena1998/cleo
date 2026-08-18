package sessionlifecycle_test

import (
	"strings"
	"testing"

	"github.com/dhruvsaxena1998/cleo/internal/sessionlifecycle"
)

func TestPruneRemovesCleanWorktreesWithRecords(t *testing.T) {
	h := newWorktreeHarness(t)
	sess := killedWorktreeSession(t, h, "finished")

	result, err := h.Lifecycle.Prune(sessionlifecycle.PruneInput{
		ProjectID: h.ProjectID,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Pruned) != 1 || result.Pruned[0] != sess.ID {
		t.Fatalf("pruned = %v, want [%q]", result.Pruned, sess.ID)
	}
	if len(h.Worktree.removed) != 1 {
		t.Fatalf("worktree removals = %#v, want one", h.Worktree.removed)
	}
	got := h.Worktree.removed[0]
	if got.Dir != sess.WorktreePath || got.Force || got.DeleteBranch != "" {
		t.Fatalf("removal = %#v, want unforced, branch kept", got)
	}
	if _, err := h.State.Get(sess.ID); err == nil {
		t.Fatal("record should be gone after prune")
	}
}

func TestPruneSkipsDirtyWorktreeSessionKeepingRecord(t *testing.T) {
	h := newWorktreeHarness(t)
	dirty := killedWorktreeSession(t, h, "dirty")
	clean := killedWorktreeSession(t, h, "clean")
	h.Worktree.dirty = map[string]bool{dirty.WorktreePath: true}

	result, err := h.Lifecycle.Prune(sessionlifecycle.PruneInput{
		ProjectID: h.ProjectID,
	})
	if err != nil {
		t.Fatal(err)
	}

	if len(result.Pruned) != 1 || result.Pruned[0] != clean.ID {
		t.Fatalf("pruned = %v, want only the clean session", result.Pruned)
	}
	if len(result.Warnings) != 1 || !strings.Contains(result.Warnings[0].Error(), dirty.WorktreePath) {
		t.Fatalf("warnings = %v, want one naming the dirty worktree", result.Warnings)
	}
	if _, err := h.State.Get(dirty.ID); err != nil {
		t.Fatalf("dirty session record must be kept, got err=%v", err)
	}
	if len(h.Worktree.removed) != 1 || h.Worktree.removed[0].Dir != clean.WorktreePath {
		t.Fatalf("removals = %#v, want only the clean worktree", h.Worktree.removed)
	}
}

func TestPruneForceRemovesDirtyWorktrees(t *testing.T) {
	h := newWorktreeHarness(t)
	dirty := killedWorktreeSession(t, h, "dirty")
	h.Worktree.dirty = map[string]bool{dirty.WorktreePath: true}

	result, err := h.Lifecycle.Prune(sessionlifecycle.PruneInput{
		ProjectID: h.ProjectID,
		Force:     true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Pruned) != 1 || result.Pruned[0] != dirty.ID {
		t.Fatalf("pruned = %v, want the dirty session", result.Pruned)
	}
	if len(h.Worktree.removed) != 1 || !h.Worktree.removed[0].Force {
		t.Fatalf("removals = %#v, want one forced", h.Worktree.removed)
	}
	if h.Worktree.removed[0].DeleteBranch != "" {
		t.Fatal("even forced prune never deletes the branch")
	}
}

func TestPruneDryRunTouchesNoWorktrees(t *testing.T) {
	h := newWorktreeHarness(t)
	sess := killedWorktreeSession(t, h, "preview")

	result, err := h.Lifecycle.Prune(sessionlifecycle.PruneInput{
		ProjectID: h.ProjectID,
		DryRun:    true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Pruned) != 1 || result.Pruned[0] != sess.ID {
		t.Fatalf("dry-run candidates = %v", result.Pruned)
	}
	if len(h.Worktree.removed) != 0 {
		t.Fatalf("dry run removed worktrees: %#v", h.Worktree.removed)
	}
	if _, err := h.State.Get(sess.ID); err != nil {
		t.Fatalf("dry run deleted the record: %v", err)
	}
}
