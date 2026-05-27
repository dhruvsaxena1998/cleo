package sessionlifecycle

import (
	"errors"
	"fmt"

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
// Returns ErrSessionNotFound when the Session does not exist.
func (l *Lifecycle) PrepareAttach(sessionID string) (PrepareAttachResult, error) {
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

	// Check tmux liveness — if the tmux session is gone, mark it dead.
	if checker, ok := l.tmux.(interface{ HasSession(string) (bool, error) }); ok {
		live, err := checker.HasSession(sessionID)
		if err != nil || !live {
			updated, applyErr := l.state.ApplySynthetic(sessionID, state.EvDead, "")
			if applyErr != nil {
				return PrepareAttachResult{}, applyErr
			}
			return PrepareAttachResult{
				Session: updated,
				Action:  AttachMarkedDead,
			}, nil
		}
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

// SetFocused sets or clears the focus state for a Session. This is a no-op
// when the focus store is not configured.
func (l *Lifecycle) SetFocused(sessionID string, focused bool) {
	if l.focus == nil {
		return
	}
	_ = l.focus.Set(sessionID, focused)
}
