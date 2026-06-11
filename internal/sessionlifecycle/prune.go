package sessionlifecycle

import (
	"fmt"
	"sort"

	"github.com/dhruvsaxena1998/cleo/internal/events"
	"github.com/dhruvsaxena1998/cleo/internal/state"
)

// PruneInput describes which finished Sessions to prune.
type PruneInput struct {
	ProjectID   string // if non-empty and AllProjects is false, only prune this project
	Keep        int    // keep N most recent finished Sessions per project; 0 = keep none
	AllProjects bool   // if true, ignore ProjectID and prune across all projects
	DryRun      bool   // if true, return candidates without archiving or deleting
	Force       bool   // if true, remove dirty Worktrees too instead of skipping their Sessions
}

// PruneResult describes the outcome of a prune operation.
type PruneResult struct {
	Pruned   []string // IDs of pruned Sessions
	Warnings []error  // archive/warning failures
}

// Prune returns prune candidates and optionally archives event logs and deletes
// Session records. When DryRun is true only candidates are returned.
func (l *Lifecycle) Prune(input PruneInput) (PruneResult, error) {
	sessions, err := l.state.List()
	if err != nil {
		return PruneResult{}, err
	}

	// Filter to finished sessions and apply project filter.
	var candidates []state.Session
	for _, s := range sessions {
		if !s.State.IsFinished() {
			continue
		}
		if !input.AllProjects && input.ProjectID != "" && s.ProjectID != input.ProjectID {
			continue
		}
		candidates = append(candidates, s)
	}

	// Group by project, sort by LastEventAt descending, and apply keep count.
	byProj := map[string][]state.Session{}
	for _, s := range candidates {
		byProj[s.ProjectID] = append(byProj[s.ProjectID], s)
	}

	var toPrune []state.Session
	for _, ss := range byProj {
		sort.Slice(ss, func(i, j int) bool {
			return ss[i].LastEventAt.After(ss[j].LastEventAt)
		})
		for i, s := range ss {
			if i < input.Keep {
				continue
			}
			toPrune = append(toPrune, s)
		}
	}

	if input.DryRun {
		return PruneResult{Pruned: sessionIDs(toPrune)}, nil
	}

	// Perform prune: remove Worktrees, archive event logs, and delete Session
	// records. A dirty Worktree skips its whole Session — worktree and record
	// live and die together (ADR 0005) — unless Force is set. The branch is
	// never deleted.
	var pruned []string
	var warnings []error
	for _, s := range toPrune {
		if s.HasWorktree() {
			if removeErr := l.removeWorktreeUnlessDirty(s, input.Force); removeErr != nil {
				warnings = append(warnings, removeErr)
				continue
			}
		}
		if err := events.Archive(l.paths.EventsLog(s.ID), l.paths.ArchiveDir()); err != nil {
			warnings = append(warnings, fmt.Errorf("archive event log for %s: %w", s.ID, err))
		}
		if err := l.state.Delete(s.ID); err != nil {
			return PruneResult{}, fmt.Errorf("delete session %s: %w", s.ID, err)
		}
		pruned = append(pruned, s.ID)
	}

	return PruneResult{Pruned: pruned, Warnings: warnings}, nil
}

func sessionIDs(ss []state.Session) []string {
	out := make([]string, 0, len(ss))
	for _, s := range ss {
		out = append(out, s.ID)
	}
	return out
}
