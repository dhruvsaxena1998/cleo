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
	// WorktreePath is set when the Session is worktree-backed: the worktree
	// (and the record, marked dead) were kept for post-mortem inspection.
	WorktreePath string
}

// Kill best-effort kills the tmux session when one is still live, then
// removes the Session record — except for worktree-backed Sessions, whose
// record is kept (marked dead) because the Worktree and the record live and
// die together (ADR 0005) and kill is exactly when post-mortem inspection
// matters. A session whose tmux session has already exited (e.g. a dead
// session) skips the tmux kill without a warning, since its absence from tmux
// is the very state a kill aims for. Returns an error wrapping
// ErrSessionNotFound when the Session does not exist.
func (l *Lifecycle) Kill(sessionID string) (KillResult, error) {
	sess, err := l.state.Get(sessionID)
	if err != nil {
		if errors.Is(err, state.ErrSessionNotFound) {
			return KillResult{}, fmt.Errorf("%w: %s", ErrSessionNotFound, sessionID)
		}
		return KillResult{}, err
	}

	// Only ask tmux to kill a session that is actually live. A dead session has
	// no tmux session, so `tmux kill-session` exits non-zero ("can't find
	// session") and would surface a spurious warning — even though "no tmux
	// session" is exactly the post-condition a kill aims for. Treat a confirmed
	// absence as success; if liveness can't be determined, fall through to a
	// best-effort kill so genuine failures still surface.
	var warning error
	if live, err := l.tmux.HasSession(sessionID); err != nil || live {
		if err := l.tmux.Kill(sessionID); err != nil {
			warning = fmt.Errorf("tmux kill failed: %w", err)
		}
	}

	if sess.HasWorktree() {
		if _, err := l.state.ApplySynthetic(sessionID, state.EvDead, ""); err != nil {
			return KillResult{}, fmt.Errorf("mark session dead: %w", err)
		}
		return KillResult{SessionID: sessionID, Warning: warning, WorktreePath: sess.WorktreePath}, nil
	}

	if err := l.state.Delete(sessionID); err != nil {
		return KillResult{}, fmt.Errorf("delete session state: %w", err)
	}

	return KillResult{SessionID: sessionID, Warning: warning}, nil
}
