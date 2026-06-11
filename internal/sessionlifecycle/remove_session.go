package sessionlifecycle

import (
	"errors"
	"fmt"
	"path/filepath"

	"github.com/dhruvsaxena1998/cleo/internal/events"
	"github.com/dhruvsaxena1998/cleo/internal/state"
)

var (
	// ErrWorktreeDirty is returned when removal would destroy uncommitted work
	// in a Session's Worktree. The worktree and the record are both kept —
	// they live and die together (ADR 0005). Force overrides.
	ErrWorktreeDirty = errors.New("worktree has uncommitted changes")
	// ErrSessionActive is returned when removing a Session that is still
	// running. Kill it first, or force.
	ErrSessionActive = errors.New("session is active")
)

// RemoveSessionInput describes one Session record to remove.
type RemoveSessionInput struct {
	SessionID string
	Force     bool // remove even when active or when the worktree is dirty
}

// RemoveSessionResult describes the outcome of removing a Session.
type RemoveSessionResult struct {
	SessionID string
	Removed   bool
	Warnings  []error
}

// RemoveSession removes a Session record together with its Worktree, honoring
// the lifetime invariant: the two live and die as a unit. The branch is never
// deleted, so committed work stays reachable.
func (l *Lifecycle) RemoveSession(input RemoveSessionInput) (RemoveSessionResult, error) {
	sess, err := l.state.Get(input.SessionID)
	if err != nil {
		if errors.Is(err, state.ErrSessionNotFound) {
			return RemoveSessionResult{}, fmt.Errorf("%w: %s", ErrSessionNotFound, input.SessionID)
		}
		return RemoveSessionResult{}, err
	}

	var warnings []error

	if !sess.State.IsFinished() {
		if !input.Force {
			return RemoveSessionResult{}, fmt.Errorf("%w: %s — kill it first or use force", ErrSessionActive, sess.ID)
		}
		if err := l.tmux.Kill(sess.ID); err != nil {
			warnings = append(warnings, fmt.Errorf("kill %s: %w", sess.ID, err))
		}
	}

	if sess.HasWorktree() {
		if err := l.removeWorktreeUnlessDirty(sess, input.Force); err != nil {
			return RemoveSessionResult{}, err
		}
	}

	if err := events.Archive(l.paths.EventsLog(sess.ID), l.paths.ArchiveDir()); err != nil {
		warnings = append(warnings, fmt.Errorf("archive event log for %s: %w", sess.ID, err))
	}
	if err := l.state.Delete(sess.ID); err != nil {
		return RemoveSessionResult{}, fmt.Errorf("delete session state: %w", err)
	}

	return RemoveSessionResult{SessionID: sess.ID, Removed: true, Warnings: warnings}, nil
}

// worktreeProjectPath is the inverse of worktreeDir: the Project path that a
// worktree directory was placed under (<project>/.cleo/worktrees/<key>).
func worktreeProjectPath(dir string) string {
	return filepath.Dir(filepath.Dir(filepath.Dir(dir)))
}
