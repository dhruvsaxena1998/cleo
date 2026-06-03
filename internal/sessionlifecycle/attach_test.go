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

// A tmux query *error* (e.g. tmux not on PATH when attaching over SSH) is NOT
// proof the session died. PrepareAttach must surface the error and leave the
// record intact — never mark a possibly-live session dead, since dead is a hard
// terminal state and the mistake is irreversible.
func TestPrepareAttachOnTmuxErrorDoesNotMarkDead(t *testing.T) {
	h := newTestHarness(t)
	pid := h.addProject(t, "myapp")
	sid := h.seedSession(t, pid, state.Running, "live-but-unreachable")
	h.Tmux.hasSessionErr = errors.New(`exec: "tmux": executable file not found in $PATH`)

	_, err := h.Lifecycle.PrepareAttach(sid)
	if err == nil {
		t.Fatal("expected an error when tmux liveness cannot be determined")
	}
	// Crucially, the session must NOT have been marked dead.
	stored, gerr := h.State.Get(sid)
	if gerr != nil {
		t.Fatal(gerr)
	}
	if stored.State != state.Running {
		t.Fatalf("session state = %s, want %s (a tmux error must not mark a session dead)", stored.State, state.Running)
	}
}

// Attach wraps PrepareAttach, so the same guarantee holds end-to-end: a tmux
// error yields an error and no attach command, with state untouched.
func TestAttachOnTmuxErrorReturnsErrorWithoutMarkingDead(t *testing.T) {
	h := newTestHarness(t)
	pid := h.addProject(t, "myapp")
	sid := h.seedSession(t, pid, state.Running, "live-but-unreachable")
	h.Tmux.hasSessionErr = errors.New(`exec: "tmux": executable file not found in $PATH`)

	plan, err := h.Lifecycle.Attach(sid)
	if err == nil {
		t.Fatal("expected an error when tmux liveness cannot be determined")
	}
	if plan.Cmd != nil {
		t.Fatal("a tmux error must not yield an attach command")
	}
	stored, gerr := h.State.Get(sid)
	if gerr != nil {
		t.Fatal(gerr)
	}
	if stored.State != state.Running {
		t.Fatalf("session state = %s, want %s (a tmux error must not mark a session dead)", stored.State, state.Running)
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

// Attach wraps PrepareAttach: a live, running Session is attachable, so the plan
// carries the ready action plus a command to run and a Done teardown.
func TestAttachOnRunningSessionReturnsReadyPlanWithCommand(t *testing.T) {
	h := newTestHarness(t)
	pid := h.addProject(t, "myapp")
	sid := h.seedSession(t, pid, state.Running, "running-session")
	h.Tmux.live = map[string]bool{sid: true}

	plan, err := h.Lifecycle.Attach(sid)
	if err != nil {
		t.Fatal(err)
	}
	if plan.Action != sessionlifecycle.AttachReady {
		t.Fatalf("expected AttachReady, got %v", plan.Action)
	}
	if plan.Cmd == nil {
		t.Fatal("ready session should yield a non-nil attach command")
	}
	if plan.Done == nil {
		t.Fatal("ready session should yield a non-nil Done teardown")
	}
}

// A dead Session is un-attachable: the plan reports blocked and carries no
// command for the caller to run.
func TestAttachOnDeadSessionIsBlockedWithNoCommand(t *testing.T) {
	h := newTestHarness(t)
	pid := h.addProject(t, "myapp")
	sid := h.seedSession(t, pid, state.Dead, "dead-session")

	plan, err := h.Lifecycle.Attach(sid)
	if err != nil {
		t.Fatal(err)
	}
	if plan.Action != sessionlifecycle.AttachBlocked {
		t.Fatalf("expected AttachBlocked, got %v", plan.Action)
	}
	if plan.Cmd != nil {
		t.Fatal("blocked session must not yield an attach command")
	}
	if plan.Done != nil {
		t.Fatal("blocked session must not yield a Done teardown")
	}
}

// When the tmux session is gone the plan reports marked-dead, transitions
// state, and carries no command.
func TestAttachOnMissingTmuxIsMarkedDeadWithNoCommand(t *testing.T) {
	h := newTestHarness(t)
	pid := h.addProject(t, "myapp")
	sid := h.seedSession(t, pid, state.Running, "ghost-session")
	h.Tmux.live = map[string]bool{sid: false}

	plan, err := h.Lifecycle.Attach(sid)
	if err != nil {
		t.Fatal(err)
	}
	if plan.Action != sessionlifecycle.AttachMarkedDead {
		t.Fatalf("expected AttachMarkedDead, got %v", plan.Action)
	}
	if plan.Cmd != nil {
		t.Fatal("marked-dead session must not yield an attach command")
	}
	stored, err := h.State.Get(sid)
	if err != nil {
		t.Fatal(err)
	}
	if stored.State != state.Dead {
		t.Fatalf("session state = %s, want %s", stored.State, state.Dead)
	}
}

// A completed-but-live Session is revived and is attachable, so the plan
// carries a command.
func TestAttachOnCompletedButLiveSessionRevivesWithCommand(t *testing.T) {
	h := newTestHarness(t)
	pid := h.addProject(t, "myapp")
	sid := h.seedSession(t, pid, state.Completed, "completed-live")
	h.Tmux.live = map[string]bool{sid: true}

	plan, err := h.Lifecycle.Attach(sid)
	if err != nil {
		t.Fatal(err)
	}
	if plan.Action != sessionlifecycle.AttachRevived {
		t.Fatalf("expected AttachRevived, got %v", plan.Action)
	}
	if plan.Cmd == nil {
		t.Fatal("revived session should yield a non-nil attach command")
	}
}

// The plan's Done teardown is the focus-clear half of the bracket: attaching a
// ready Session sets focus, and running Done clears it.
func TestAttachDoneClearsFocus(t *testing.T) {
	h := newTestHarness(t)
	pid := h.addProject(t, "myapp")
	sid := h.seedSession(t, pid, state.Running, "focus-session")
	h.Tmux.live = map[string]bool{sid: true}

	plan, err := h.Lifecycle.Attach(sid)
	if err != nil {
		t.Fatal(err)
	}
	if !h.Focus.IsFocused(sid) {
		t.Fatal("expected focus set after Attach on a ready session")
	}
	plan.Done()
	if h.Focus.IsFocused(sid) {
		t.Fatal("expected focus cleared after running plan.Done()")
	}
}
