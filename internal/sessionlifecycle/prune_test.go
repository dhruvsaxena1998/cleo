package sessionlifecycle_test

import (
	"testing"
	"time"

	"github.com/dhruvsaxena1998/cleo/internal/sessionlifecycle"
	"github.com/dhruvsaxena1998/cleo/internal/state"
)

func TestPruneSelectsOnlyFinishedSessions(t *testing.T) {
	h := newTestHarness(t)
	pid := h.addProject(t, "myapp")

	h.seedSession(t, pid, state.Completed, "finished-1")
	h.seedSession(t, pid, state.Errored, "finished-2")
	h.seedSession(t, pid, state.Dead, "finished-3")
	h.seedSession(t, pid, state.Running, "active")

	result, err := h.Lifecycle.Prune(sessionlifecycle.PruneInput{
		ProjectID: pid,
		Keep:      0,
		DryRun:    true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Pruned) != 3 {
		t.Fatalf("expected 3 pruned (completed, errored, dead), got %d: %v", len(result.Pruned), result.Pruned)
	}
}

func TestPruneWithProjectFilterOnlyPrunesThatProject(t *testing.T) {
	h := newTestHarness(t)
	pidA := h.addProject(t, "project-a")
	pidB := h.addProject(t, "project-b")

	h.seedSession(t, pidA, state.Completed, "a-session")
	h.seedSession(t, pidB, state.Completed, "b-session")

	result, err := h.Lifecycle.Prune(sessionlifecycle.PruneInput{
		ProjectID: pidA,
		Keep:      0,
		DryRun:    true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Pruned) != 1 {
		t.Fatalf("expected 1 pruned session from project-a, got %d: %v", len(result.Pruned), result.Pruned)
	}
}

func TestPruneWithKeepCountKeepsNMostRecentPerProject(t *testing.T) {
	h := newTestHarness(t)
	pid := h.addProject(t, "myapp")
	now := time.Now()

	// oldest, middle, newest - seedSession writes LastEventAt as zero, so we need
	// to set it explicitly after creation.
	oldest := h.seedSession(t, pid, state.Completed, "oldest")
	middle := h.seedSession(t, pid, state.Completed, "middle")
	newest := h.seedSession(t, pid, state.Completed, "newest")

	// Set LastEventAt explicitly.
	for _, s := range []struct {
		id    string
		delta time.Duration
	}{{oldest, -3 * time.Minute}, {middle, -2 * time.Minute}, {newest, -1 * time.Minute}} {
		sess, err := h.State.Get(s.id)
		if err != nil {
			t.Fatal(err)
		}
		sess.LastEventAt = now.Add(s.delta)
		if err := h.State.Put(sess); err != nil {
			t.Fatal(err)
		}
	}

	result, err := h.Lifecycle.Prune(sessionlifecycle.PruneInput{
		ProjectID: pid,
		Keep:      1,
		DryRun:    true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Pruned) != 2 {
		t.Fatalf("expected 2 pruned (keep 1 of 3), got %d: %v", len(result.Pruned), result.Pruned)
	}
	// The newest should be kept; oldest and middle should be pruned.
	for _, id := range result.Pruned {
		if id == newest {
			t.Fatalf("newest session should not be pruned (keep=1), but it was pruned")
		}
	}
}

func TestPruneArchivesEventLogsBeforeDeletingSessionRecords(t *testing.T) {
	h := newTestHarness(t)
	pid := h.addProject(t, "myapp")
	sid := h.seedSession(t, pid, state.Completed, "to-prune")
	h.writeEventLog(t, sid)

	result, err := h.Lifecycle.Prune(sessionlifecycle.PruneInput{
		ProjectID: pid,
		Keep:      0,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Pruned) != 1 {
		t.Fatalf("expected 1 pruned session, got %d", len(result.Pruned))
	}
	if len(result.Warnings) != 0 {
		t.Fatalf("expected no warnings, got %v", result.Warnings)
	}

	// Event log should be archived (active log deleted, archive present).
	h.assertEventLogArchived(t, sid)
	h.assertEventLogDeleted(t, sid)

	// Session record should be deleted.
	if _, err := h.State.Get(sid); err == nil {
		t.Fatal("session should be deleted after prune")
	}
}

func TestPruneWithZeroKeepPrunesAllFinishedSessions(t *testing.T) {
	h := newTestHarness(t)
	pid := h.addProject(t, "myapp")

	h.seedSession(t, pid, state.Completed, "first")
	h.seedSession(t, pid, state.Completed, "second")

	result, err := h.Lifecycle.Prune(sessionlifecycle.PruneInput{
		ProjectID: pid,
		Keep:      0,
		DryRun:    true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Pruned) != 2 {
		t.Fatalf("expected 2 pruned (keep=0), got %d: %v", len(result.Pruned), result.Pruned)
	}
}
