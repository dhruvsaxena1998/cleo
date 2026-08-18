package sessionlifecycle_test

import (
	"testing"

	"github.com/dhruvsaxena1998/cleo/internal/sessionlifecycle"
	"github.com/dhruvsaxena1998/cleo/internal/state"
)

// Kill is exactly when post-mortem inspection matters most, so for a worktree
// Session it must keep both halves of the lifetime invariant: the worktree
// stays on disk and the Session record stays in the store (marked dead).
// Removal is rm/prune's job.
func TestKillWorktreeSessionKeepsRecordAndWorktree(t *testing.T) {
	h := newWorktreeHarness(t)
	result, err := h.Lifecycle.Create(sessionlifecycle.CreateInput{
		Agent:     "claude",
		Name:      "post-mortem",
		ProjectID: h.ProjectID,
		Worktree:  boolPtr(true),
	})
	if err != nil {
		t.Fatal(err)
	}
	sid := result.Session.ID

	killResult, err := h.Lifecycle.Kill(sid)
	if err != nil {
		t.Fatal(err)
	}

	if len(h.Tmux.killed) != 1 || h.Tmux.killed[0] != sid {
		t.Fatalf("tmux killed = %v, want [%q]", h.Tmux.killed, sid)
	}
	for _, call := range h.Worktree.calls {
		if call == "remove" {
			t.Fatal("kill must never remove the worktree")
		}
	}

	stored, err := h.State.Get(sid)
	if err != nil {
		t.Fatalf("worktree session record should survive kill, got err=%v", err)
	}
	if stored.State != state.Dead {
		t.Fatalf("killed worktree session state = %s, want dead", stored.State)
	}
	if killResult.WorktreePath != result.Session.WorktreePath {
		t.Fatalf("KillResult.WorktreePath = %q, want %q so callers can say where the work is",
			killResult.WorktreePath, result.Session.WorktreePath)
	}
}
