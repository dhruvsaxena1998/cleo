package worktree

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// excludeLine matches Cleo's whole footprint at any depth, which also covers
// monorepo Projects below the repo root. Written to the repo-local
// .git/info/exclude — never .gitignore — so no tracked file ever changes.
const excludeLine = ".cleo/"

// Client runs git CLI commands. The zero value is ready to use.
type Client struct{}

func NewClient() *Client { return &Client{} }

// Create makes a full-repo worktree at o.Dir on a new branch o.Branch starting
// from o.Base (HEAD when empty). It fails — before anything is created — on a
// non-git project, an unborn HEAD, or an unresolvable base.
func (c *Client) Create(o CreateOpts) (Created, error) {
	root, err := repoRoot(o.ProjectPath)
	if err != nil {
		return Created{}, err
	}

	base := o.Base
	if base == "" {
		base = "HEAD"
	}
	if _, err := gitOut(root, "rev-parse", "--verify", "--quiet", base+"^{commit}"); err != nil {
		if o.Base == "" {
			return Created{}, fmt.Errorf("repository at %s has no commits yet (unborn HEAD); commit once before spawning a worktree session", root)
		}
		return Created{}, fmt.Errorf("base %q does not resolve to a commit in %s", o.Base, root)
	}

	if err := os.MkdirAll(filepath.Dir(o.Dir), 0o755); err != nil {
		return Created{}, err
	}
	if out, err := gitOut(root, "worktree", "add", "-b", o.Branch, o.Dir, base); err != nil {
		return Created{}, fmt.Errorf("git worktree add: %v: %s", err, out)
	}

	// Monorepo mapping: when the Project is a subdirectory of its repo, the
	// session works in the matching subdirectory inside the worktree. Resolve
	// symlinks on both sides first — git reports the toplevel with symlinks
	// resolved (e.g. /var → /private/var on macOS), the project path may not be.
	projectResolved, err := filepath.EvalSymlinks(o.ProjectPath)
	if err != nil {
		return Created{}, err
	}
	rel, err := filepath.Rel(root, projectResolved)
	if err != nil || strings.HasPrefix(rel, "..") {
		return Created{}, fmt.Errorf("project path %s is not inside repository %s", o.ProjectPath, root)
	}
	return Created{CWD: filepath.Join(o.Dir, rel)}, nil
}

// Remove deletes the worktree at o.Dir, leaning on git's own safety model:
// `git worktree remove` refuses dirty worktrees unless forced. A directory the
// user already deleted manually is tolerated — the stale metadata is pruned
// and the removal counts as done. The branch is only deleted when
// o.DeleteBranch names it (spawn rollback).
func (c *Client) Remove(o RemoveOpts) error {
	root, err := repoRoot(o.ProjectPath)
	if err != nil {
		return err
	}

	if _, statErr := os.Stat(o.Dir); os.IsNotExist(statErr) {
		if out, err := gitOut(root, "worktree", "prune"); err != nil {
			return fmt.Errorf("git worktree prune: %v: %s", err, out)
		}
	} else {
		args := []string{"worktree", "remove"}
		if o.Force {
			args = append(args, "--force")
		}
		args = append(args, o.Dir)
		if out, err := gitOut(root, args...); err != nil {
			return fmt.Errorf("git worktree remove: %v: %s", err, out)
		}
	}

	if o.DeleteBranch != "" {
		if out, err := gitOut(root, "branch", "-D", o.DeleteBranch); err != nil {
			return fmt.Errorf("git branch -D %s: %v: %s", o.DeleteBranch, err, out)
		}
	}
	return nil
}

// IsDirty reports uncommitted work (including untracked files) in the worktree
// at dir. A missing directory reports clean: the user deleted it manually and
// cleanup should proceed.
func (c *Client) IsDirty(dir string) (bool, error) {
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return false, nil
	}
	out, err := gitOut(dir, "status", "--porcelain")
	if err != nil {
		return false, fmt.Errorf("git status in %s: %v: %s", dir, err, out)
	}
	return strings.TrimSpace(out) != "", nil
}

// EnsureExcluded idempotently appends the `.cleo/` line to the repo-local
// .git/info/exclude of the repo containing projectPath.
func (c *Client) EnsureExcluded(projectPath string) error {
	commonDir, err := gitOut(projectPath, "rev-parse", "--path-format=absolute", "--git-common-dir")
	if err != nil {
		return fmt.Errorf("locate git dir for %s: %v: %s", projectPath, err, commonDir)
	}
	excludePath := filepath.Join(strings.TrimSpace(commonDir), "info", "exclude")

	existing, err := os.ReadFile(excludePath)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	for _, line := range strings.Split(string(existing), "\n") {
		if strings.TrimSpace(line) == excludeLine {
			return nil
		}
	}

	if err := os.MkdirAll(filepath.Dir(excludePath), 0o755); err != nil {
		return err
	}
	content := string(existing)
	if content != "" && !strings.HasSuffix(content, "\n") {
		content += "\n"
	}
	content += excludeLine + "\n"
	return os.WriteFile(excludePath, []byte(content), 0o644)
}

func repoRoot(projectPath string) (string, error) {
	out, err := gitOut(projectPath, "rev-parse", "--show-toplevel")
	if err != nil {
		return "", fmt.Errorf("%s is not inside a git repository (worktree sessions need one)", projectPath)
	}
	return strings.TrimSpace(out), nil
}

func gitOut(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	return strings.TrimSpace(string(out)), err
}
