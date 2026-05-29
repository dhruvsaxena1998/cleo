package sessionlifecycle

import (
	"errors"
	"fmt"

	"github.com/dhruvsaxena1998/cleo/internal/events"
)

// ErrActiveSessionsBlock is returned by RemoveProjectSessions when active
// Sessions exist and Force is false.
var ErrActiveSessionsBlock = errors.New("active sessions block removal")

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

	var active, finished []string
	for _, s := range sessions {
		if s.ProjectID != input.ProjectID {
			continue
		}
		if s.State.IsFinished() {
			finished = append(finished, s.ID)
		} else {
			active = append(active, s.ID)
		}
	}

	if !input.Force && len(active) > 0 {
		return RemoveProjectSessionsResult{
			ActiveCount:   len(active),
			FinishedCount: len(finished),
		}, ErrActiveSessionsBlock
	}

	var warnings []error
	var removed []string

	// Best-effort kill active Sessions.
	for _, id := range active {
		if err := l.tmux.Kill(id); err != nil {
			warnings = append(warnings, fmt.Errorf("kill %s: %w", id, err))
		}
	}

	// Archive event logs and delete Session records.
	allIDs := append(active, finished...)
	for _, id := range allIDs {
		if err := events.Archive(l.paths.EventsLog(id), l.paths.ArchiveDir()); err != nil {
			warnings = append(warnings, fmt.Errorf("archive event log for %s: %w", id, err))
		}
		if err := l.state.Delete(id); err != nil {
			return RemoveProjectSessionsResult{}, fmt.Errorf("delete session %s: %w", id, err)
		}
		removed = append(removed, id)
	}

	return RemoveProjectSessionsResult{
		RemovedSessionIDs: removed,
		Warnings:          warnings,
		ActiveCount:       len(active),
		FinishedCount:     len(finished),
	}, nil
}
