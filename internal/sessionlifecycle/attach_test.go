package sessionlifecycle_test

import (
	"errors"
	"testing"

	"github.com/dhruvsaxena1998/cleo/internal/sessionlifecycle"
	"github.com/dhruvsaxena1998/cleo/internal/state"
)

func TestPrepareAttachBlocksDeadSession(t *testing.T) {
	h := newTestHarness(t)
	pid := h.addProject(t, "myapp")
	sid := h.seedSession(t, pid, state.Dead, "dead-session")

	result, err := h.Lifecycle.PrepareAttach(sid)
	if err != nil {
		t.Fatal(err)
	}
	if result.Action != sessionlifecycle.AttachBlocked {
		t.Fatalf("expected AttachBlocked, got %v", result.Action)
	}
}

func TestPrepareAttachBlocksErroredSession(t *testing.T) {
	h := newTestHarness(t)
	pid := h.addProject(t, "myapp")
	sid := h.seedSession(t, pid, state.Errored, "errored-session")

	result, err := h.Lifecycle.PrepareAttach(sid)
	if err != nil {
		t.Fatal(err)
	}
	if result.Action != sessionlifecycle.AttachBlocked {
		t.Fatalf("expected AttachBlocked, got %v", result.Action)
	}
}

func TestPrepareAttachOnMissingTmuxMarksSessionDead(t *testing.T) {
	h := newTestHarness(t)
	pid := h.addProject(t, "myapp")
	sid := h.seedSession(t, pid, state.Running, "ghost-session")
	// The fake reports no tmux session for this ID.
	h.Tmux.live = map[string]bool{sid: false}

	result, err := h.Lifecycle.PrepareAttach(sid)
	if err != nil {
		t.Fatal(err)
	}
	if result.Action != sessionlifecycle.AttachMarkedDead {
		t.Fatalf("expected AttachMarkedDead, got %v", result.Action)
	}
	// Session state should be updated to dead.
	stored, err := h.State.Get(sid)
	if err != nil {
		t.Fatal(err)
	}
	if stored.State != state.Dead {
		t.Fatalf("session state = %s, want %s", stored.State, state.Dead)
	}
}

func TestPrepareAttachOnCompletedButLiveSessionRevives(t *testing.T) {
	h := newTestHarness(t)
	pid := h.addProject(t, "myapp")
	sid := h.seedSession(t, pid, state.Completed, "completed-live")
	h.Tmux.live = map[string]bool{sid: true}

	result, err := h.Lifecycle.PrepareAttach(sid)
	if err != nil {
		t.Fatal(err)
	}
	if result.Action != sessionlifecycle.AttachRevived {
		t.Fatalf("expected AttachRevived, got %v", result.Action)
	}
	// Session state should be revived (to Idle per state machine).
	stored, err := h.State.Get(sid)
	if err != nil {
		t.Fatal(err)
	}
	if stored.State != state.Idle {
		t.Fatalf("session state = %s, want %s (Idle after revive)", stored.State, state.Idle)
	}
}

func TestPrepareAttachOnRunningSessionReturnsReady(t *testing.T) {
	h := newTestHarness(t)
	pid := h.addProject(t, "myapp")
	sid := h.seedSession(t, pid, state.Running, "running-session")
	h.Tmux.live = map[string]bool{sid: true}

	result, err := h.Lifecycle.PrepareAttach(sid)
	if err != nil {
		t.Fatal(err)
	}
	if result.Action != sessionlifecycle.AttachReady {
		t.Fatalf("expected AttachReady, got %v", result.Action)
	}
	// Session state should be unchanged.
	stored, err := h.State.Get(sid)
	if err != nil {
		t.Fatal(err)
	}
	if stored.State != state.Running {
		t.Fatalf("session state changed from Running to %s", stored.State)
	}
}

func TestPrepareAttachUnknownSessionReturnsErrSessionNotFound(t *testing.T) {
	h := newTestHarness(t)
	h.addProject(t, "myapp")

	_, err := h.Lifecycle.PrepareAttach("cleo-nonexistent-project-agent-session")
	if err == nil {
		t.Fatal("expected error for unknown session")
	}
	if !errors.Is(err, sessionlifecycle.ErrSessionNotFound) {
		t.Fatalf("error = %v, want ErrSessionNotFound", err)
	}
}
