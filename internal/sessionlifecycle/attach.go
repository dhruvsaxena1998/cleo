package sessionlifecycle

import (
	"errors"
	"fmt"
	"os/exec"

	"github.com/dhruvsaxena1998/cleo/internal/state"
)

// AttachAction indicates what happened when preparing a Session for attach.
type AttachAction int

const (
	AttachBlocked    AttachAction = iota // Session is dead or errored
	AttachReady                          // tmux session is alive and attachable
	AttachRevived                        // completed Session revived via EvUserResume
	AttachMarkedDead                     // tmux session was gone, Session marked dead
)

// PrepareAttachResult describes the outcome of preparing a Session for attach.
type PrepareAttachResult struct {
	Session state.Session
	Action  AttachAction
}

// PrepareAttach validates the Session for attach, checks tmux liveness, and
// applies any needed state transitions (mark missing as dead, revive completed).
// As a side effect, it installs focus hooks and binds the detach key so that
// every attach path (CLI and TUI) gets these without duplicating the logic.
// Returns ErrSessionNotFound when the Session does not exist.
func (l *Lifecycle) PrepareAttach(sessionID string) (PrepareAttachResult, error) {
	l.installFocusHooks()
	l.bindDetachKey()
	sess, err := l.state.Get(sessionID)
	if err != nil {
		if errors.Is(err, state.ErrSessionNotFound) {
			return PrepareAttachResult{}, fmt.Errorf("%w: %s", ErrSessionNotFound, sessionID)
		}
		return PrepareAttachResult{}, err
	}

	// Block hard terminal states: dead and errored Sessions are un-attachable.
	if sess.State == state.Dead || sess.State == state.Errored {
		return PrepareAttachResult{
			Session: sess,
			Action:  AttachBlocked,
		}, nil
	}

	// Check tmux liveness. A query *error* (e.g. tmux not on PATH when attaching
	// over SSH, or an unreachable socket) is NOT proof the session died — only a
	// definitive "no such session" is. Marking dead here is irreversible (dead is
	// a hard terminal state the reconciler never revives), so on an error we
	// surface it and leave the record untouched rather than killing a live session.
	live, err := l.tmux.HasSession(sessionID)
	if err != nil {
		return PrepareAttachResult{Session: sess},
			fmt.Errorf("cannot determine whether %s is alive (is tmux installed and on PATH?): %w", sessionID, err)
	}
	if !live {
		updated, applyErr := l.state.ApplySynthetic(sessionID, state.EvDead, "")
		if applyErr != nil {
			return PrepareAttachResult{}, applyErr
		}
		return PrepareAttachResult{
			Session: updated,
			Action:  AttachMarkedDead,
		}, nil
	}

	// Revive completed-but-live sessions.
	if sess.State == state.Completed {
		updated, applyErr := l.state.Apply(sessionID, state.EvUserResume, "re-attached by user")
		if applyErr != nil {
			return PrepareAttachResult{}, applyErr
		}
		return PrepareAttachResult{
			Session: updated,
			Action:  AttachRevived,
		}, nil
	}

	return PrepareAttachResult{
		Session: sess,
		Action:  AttachReady,
	}, nil
}

// AttachPlan describes what a caller should do to attach to a Session. The
// lifecycle decides and prepares; the caller executes Cmd — terminal ownership
// differs between the CLI (blocks the terminal) and the TUI (suspends Bubble
// Tea) — and then runs Done to close the focus bracket at the right moment.
type AttachPlan struct {
	Action  AttachAction
	Session state.Session
	Cmd     *exec.Cmd // nil unless the Session is attachable (Ready or Revived)
	Done    func()    // run by the caller after Cmd exits; nil when Cmd is nil
}

// Attach prepares a Session for attach and returns a plan. It composes
// PrepareAttach, sets focus on when the Session is attachable, builds the
// command via the Tmux seam, and returns Done as the focus-clear teardown for
// the caller to run after Cmd exits (execution stays caller-owned).
func (l *Lifecycle) Attach(sessionID string) (AttachPlan, error) {
	result, err := l.PrepareAttach(sessionID)
	if err != nil {
		return AttachPlan{}, err
	}
	plan := AttachPlan{Action: result.Action, Session: result.Session}
	// Only attachable Sessions get focus, a command, and a teardown; blocked and
	// marked-dead Sessions return action-only so the caller just reports status.
	switch result.Action {
	case AttachReady, AttachRevived:
		l.SetFocused(sessionID, true)
		plan.Cmd = l.tmux.AttachCmd(sessionID)
		plan.Done = func() { l.SetFocused(sessionID, false) }
	}
	return plan, nil
}

// SetFocused sets or clears the focus state for a Session. This is a no-op
// when the focus store is not configured.
func (l *Lifecycle) SetFocused(sessionID string, focused bool) {
	if l.focus == nil {
		return
	}
	_ = l.focus.Set(sessionID, focused)
}
