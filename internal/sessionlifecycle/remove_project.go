package sessionlifecycle

import (
	"errors"
	"fmt"

	"github.com/dhruvsaxena1998/cleo/internal/events"
	"github.com/dhruvsaxena1998/cleo/internal/state"
	"github.com/dhruvsaxena1998/cleo/internal/worktree"
)

// ErrActiveSessionsBlock is returned by RemoveProjectSessions when active
// Sessions exist and Force is false.
var ErrActiveSessionsBlock = errors.New("active sessions block removal")

// ErrDirtyWorktreesBlock is returned by RemoveProjectSessions when any of the
// Project's Sessions has a Worktree with uncommitted work and Force is false.
// Project removal is all-or-nothing: the abort happens upfront, before
// anything is removed, so it never half-completes (ADR 0005).
var ErrDirtyWorktreesBlock = errors.New("dirty worktrees block removal")

// RemoveProjectSessionsInput describes which Project's Sessions to remove.
type RemoveProjectSessionsInput struct {
	ProjectID string
	Force     bool // when true, kill active Sessions best-effort
}

// RemoveProjectSessionsResult describes the outcome of removing a Project's Sessions.
type RemoveProjectSessionsResult struct {
	ActiveCount       int
	FinishedCount     int
	RemovedSessionIDs []string
	Warnings          []error
	// DirtyWorktreeSessionIDs lists the offenders when the removal aborted
	// with ErrDirtyWorktreesBlock.
	DirtyWorktreeSessionIDs []string
}

// RemoveProjectSessions classifies a Project's Sessions, best-effort kills
// active Sessions (when Force is true), archives event logs, and deletes all
// Session records. Returns ErrActiveSessionsBlock when active Sessions exist
// and Force is false.
func (l *Lifecycle) RemoveProjectSessions(input RemoveProjectSessionsInput) (RemoveProjectSessionsResult, error) {
	sessions, err := l.state.List()
	if err != nil {
		return RemoveProjectSessionsResult{}, err
	}

	var active, finished []state.Session
	for _, s := range sessions {
		if s.ProjectID != input.ProjectID {
			continue
		}
		if s.State.IsFinished() {
			finished = append(finished, s)
		} else {
			active = append(active, s)
		}
	}

	if !input.Force && len(active) > 0 {
		return RemoveProjectSessionsResult{
			ActiveCount:   len(active),
			FinishedCount: len(finished),
		}, ErrActiveSessionsBlock
	}

	// All-or-nothing: scan every Worktree upfront and abort before touching
	// anything when uncommitted work would be destroyed (ADR 0005).
	all := append(active, finished...)
	if !input.Force {
		var dirtyIDs []string
		for _, s := range all {
			if !s.HasWorktree() {
				continue
			}
			dirty, err := l.worktree.IsDirty(s.WorktreePath)
			if err != nil {
				return RemoveProjectSessionsResult{}, fmt.Errorf("check worktree %s: %w", s.WorktreePath, err)
			}
			if dirty {
				dirtyIDs = append(dirtyIDs, s.ID)
			}
		}
		if len(dirtyIDs) > 0 {
			return RemoveProjectSessionsResult{
				ActiveCount:             len(active),
				FinishedCount:           len(finished),
				DirtyWorktreeSessionIDs: dirtyIDs,
			}, ErrDirtyWorktreesBlock
		}
	}

	var warnings []error
	var removed []string

	// Best-effort kill active Sessions.
	for _, s := range active {
		if err := l.tmux.Kill(s.ID); err != nil {
			warnings = append(warnings, fmt.Errorf("kill %s: %w", s.ID, err))
		}
	}

	// Remove Worktrees, archive event logs, and delete Session records. The
	// upfront scan already cleared dirtiness, so a removal failure here is
	// exceptional: warn and keep going — the project is being unregistered
	// and a leftover directory is the recoverable outcome.
	for _, s := range all {
		if s.HasWorktree() {
			if err := l.worktree.Remove(worktree.RemoveOpts{
				ProjectPath: worktreeProjectPath(s.WorktreePath),
				Dir:         s.WorktreePath,
				Force:       input.Force,
			}); err != nil {
				warnings = append(warnings, fmt.Errorf("remove worktree %s: %w", s.WorktreePath, err))
			}
		}
		if err := events.Archive(l.paths.EventsLog(s.ID), l.paths.ArchiveDir()); err != nil {
			warnings = append(warnings, fmt.Errorf("archive event log for %s: %w", s.ID, err))
		}
		if err := l.state.Delete(s.ID); err != nil {
			return RemoveProjectSessionsResult{}, fmt.Errorf("delete session %s: %w", s.ID, err)
		}
		removed = append(removed, s.ID)
	}

	return RemoveProjectSessionsResult{
		RemovedSessionIDs: removed,
		Warnings:          warnings,
		ActiveCount:       len(active),
		FinishedCount:     len(finished),
	}, nil
}
