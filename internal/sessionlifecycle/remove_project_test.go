package sessionlifecycle_test

import (
	"errors"
	"testing"

	"github.com/dhruvsaxena1998/cleo/internal/sessionlifecycle"
	"github.com/dhruvsaxena1998/cleo/internal/state"
)

func TestRemoveProjectSessionsClassifiesActiveAndFinished(t *testing.T) {
	h := newTestHarness(t)
	pid := h.addProject(t, "myapp")
	h.seedSession(t, pid, state.Running, "active-1")
	h.seedSession(t, pid, state.Completed, "finished-1")
	h.seedSession(t, pid, state.Dead, "finished-2")

	result, err := h.Lifecycle.RemoveProjectSessions(sessionlifecycle.RemoveProjectSessionsInput{
		ProjectID: pid,
		Force:     true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.ActiveCount != 1 {
		t.Fatalf("ActiveCount = %d, want 1", result.ActiveCount)
	}
	if result.FinishedCount != 2 {
		t.Fatalf("FinishedCount = %d, want 2", result.FinishedCount)
	}
}

func TestRemoveProjectSessionsWithoutForceReturnsErrorWhenActiveExist(t *testing.T) {
	h := newTestHarness(t)
	pid := h.addProject(t, "myapp")
	h.seedSession(t, pid, state.Running, "active-1")

	_, err := h.Lifecycle.RemoveProjectSessions(sessionlifecycle.RemoveProjectSessionsInput{
		ProjectID: pid,
		Force:     false,
	})
	if err == nil {
		t.Fatal("expected error when active sessions exist and Force is false")
	}
	if !errors.Is(err, sessionlifecycle.ErrActiveSessionsBlock) {
		t.Fatalf("error = %v, want ErrActiveSessionsBlock", err)
	}

	// No sessions should be deleted.
	sessions, _ := h.State.List()
	if len(sessions) != 1 {
		t.Fatalf("expected 1 session (not deleted), got %d", len(sessions))
	}
}

func TestRemoveProjectSessionsDeletesSessionRecords(t *testing.T) {
	h := newTestHarness(t)
	pid := h.addProject(t, "myapp")
	h.seedSession(t, pid, state.Running, "active-1")
	h.seedSession(t, pid, state.Completed, "finished-1")

	result, err := h.Lifecycle.RemoveProjectSessions(sessionlifecycle.RemoveProjectSessionsInput{
		ProjectID: pid,
		Force:     true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.RemovedSessionIDs) != 2 {
		t.Fatalf("expected 2 removed, got %d", len(result.RemovedSessionIDs))
	}

	// All sessions should be deleted.
	sessions, _ := h.State.List()
	for _, s := range sessions {
		if s.ProjectID == pid {
			t.Fatalf("session %s still exists after removal", s.ID)
		}
	}
}

func TestRemoveProjectSessionsArchivesEventLogs(t *testing.T) {
	h := newTestHarness(t)
	pid := h.addProject(t, "myapp")
	sid := h.seedSession(t, pid, state.Completed, "to-remove")
	h.writeEventLog(t, sid)

	result, err := h.Lifecycle.RemoveProjectSessions(sessionlifecycle.RemoveProjectSessionsInput{
		ProjectID: pid,
		Force:     true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.RemovedSessionIDs) != 1 {
		t.Fatalf("expected 1 removed, got %d", len(result.RemovedSessionIDs))
	}

	// Event log should be archived (active deleted, archive present).
	h.assertEventLogArchived(t, sid)
	h.assertEventLogDeleted(t, sid)
}

func TestRemoveProjectSessionsTmuxKillFailureReturnsWarning(t *testing.T) {
	h := newTestHarness(t)
	pid := h.addProject(t, "myapp")
	sid := h.seedSession(t, pid, state.Running, "active-1")
	h.Tmux.killErr = errors.New("tmux refuses")

	result, err := h.Lifecycle.RemoveProjectSessions(sessionlifecycle.RemoveProjectSessionsInput{
		ProjectID: pid,
		Force:     true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Warnings) == 0 {
		t.Fatal("expected warnings for tmux kill failure")
	}
	if len(result.RemovedSessionIDs) != 1 {
		t.Fatalf("expected 1 removed despite tmux failure, got %d", len(result.RemovedSessionIDs))
	}
	// Session should still be deleted.
	if _, err := h.State.Get(sid); err == nil {
		t.Fatal("session should be deleted despite tmux kill failure")
	}
}

func TestRemoveProjectSessionsIsScopedToProject(t *testing.T) {
	h := newTestHarness(t)
	pidA := h.addProject(t, "project-a")
	pidB := h.addProject(t, "project-b")
	h.seedSession(t, pidA, state.Completed, "a-session")
	h.seedSession(t, pidB, state.Completed, "b-session")

	result, err := h.Lifecycle.RemoveProjectSessions(sessionlifecycle.RemoveProjectSessionsInput{
		ProjectID: pidA,
		Force:     true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.RemovedSessionIDs) != 1 {
		t.Fatalf("expected only 1 removed (project-a), got %d", len(result.RemovedSessionIDs))
	}
	// Project-b session should remain.
	sessions, _ := h.State.List()
	if len(sessions) != 1 {
		t.Fatalf("expected 1 remaining session (project-b), got %d", len(sessions))
	}
}
