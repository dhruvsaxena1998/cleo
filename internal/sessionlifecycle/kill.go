package sessionlifecycle

import (
	"errors"
	"fmt"

	"github.com/dhruvsaxena1998/cleo/internal/state"
)

// KillResult describes the outcome of killing a Session.
type KillResult struct {
	SessionID string
	Warning   error // non-nil when tmux kill failed; Session is still deleted
}

// Kill removes the Session record and best-effort kills the tmux session.
// Returns an error wrapping ErrSessionNotFound when the Session does not exist.
func (l *Lifecycle) Kill(sessionID string) (KillResult, error) {
	if _, err := l.state.Get(sessionID); err != nil {
		if errors.Is(err, state.ErrSessionNotFound) {
			return KillResult{}, fmt.Errorf("%w: %s", ErrSessionNotFound, sessionID)
		}
		return KillResult{}, err
	}

	var warning error
	if err := l.tmux.Kill(sessionID); err != nil {
		warning = fmt.Errorf("tmux kill failed: %w", err)
	}

	if err := l.state.Delete(sessionID); err != nil {
		return KillResult{}, fmt.Errorf("delete session state: %w", err)
	}

	return KillResult{SessionID: sessionID, Warning: warning}, nil
}
