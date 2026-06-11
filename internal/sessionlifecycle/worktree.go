package sessionlifecycle

import (
	"errors"
	"fmt"
	"path/filepath"

	"github.com/dhruvsaxena1998/cleo/internal/state"
	"github.com/dhruvsaxena1998/cleo/internal/worktree"
)

// ErrWorktreeFailed wraps worktree creation failures: non-git project, unborn
// HEAD, unresolvable base. They fail the spawn before any tmux session exists
// — never a silent fallback to the main working tree.
var ErrWorktreeFailed = errors.New("worktree creation failed")

// Worktree is the Session lifecycle's seam onto git worktrees: it names every
// worktree operation the lifecycle calls, so the compiler enforces that every
// adapter satisfies the whole contract. Declared here, by the consumer,
// mirroring the Tmux seam (ADR 0001); placement and lifetime policy is ADR
// 0005. The production adapter is internal/worktree.
type Worktree interface {
	Create(worktree.CreateOpts) (worktree.Created, error)
	Remove(worktree.RemoveOpts) error
	// IsDirty reports uncommitted work in the worktree at dir. A worktree the
	// user already deleted manually reports clean, so cleanup proceeds.
	IsDirty(dir string) (bool, error)
	// EnsureExcluded idempotently keeps `.cleo/` in the repo-local
	// .git/info/exclude of the repo containing projectPath.
	EnsureExcluded(projectPath string) error
}

// removeWorktreeUnlessDirty removes a Session's Worktree, skipping (with the
// reason as the returned error, wrapping ErrWorktreeDirty) when it holds
// uncommitted work and force is false. The Session record is the caller's to
// keep or delete — but on any error the record must be kept, so worktree and
// record keep living and dying together. The branch is never deleted.
func (l *Lifecycle) removeWorktreeUnlessDirty(s state.Session, force bool) error {
	if !force {
		dirty, err := l.worktree.IsDirty(s.WorktreePath)
		if err != nil {
			return fmt.Errorf("check worktree %s: %w", s.WorktreePath, err)
		}
		if dirty {
			return fmt.Errorf("%w: %s — session %s kept", ErrWorktreeDirty, s.WorktreePath, s.ID)
		}
	}
	if err := l.worktree.Remove(worktree.RemoveOpts{
		ProjectPath: worktreeProjectPath(s.WorktreePath),
		Dir:         s.WorktreePath,
		Force:       force,
	}); err != nil {
		return fmt.Errorf("remove worktree %s: %w", s.WorktreePath, err)
	}
	return nil
}

// worktreeKey is the Session ID minus the project prefix — unique within the
// project by construction, because bare Session names are only deduplicated
// per (project, agent).
func worktreeKey(agent, name string) string { return agent + "-" + name }

// worktreeDir is the in-project placement decided by ADR 0005.
func worktreeDir(projectPath, key string) string {
	return filepath.Join(projectPath, ".cleo", "worktrees", key)
}

func worktreeBranch(key string) string { return "cleo/wt-" + key }
