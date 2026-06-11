// Package worktree creates and removes Cleo-managed git worktrees by shelling
// out to the git CLI. It is the production adapter behind the Session
// lifecycle's Worktree seam; placement and lifetime rules are recorded in
// docs/adr/0005-worktree-placement-and-lifetime.md.
package worktree

// CreateOpts describes one worktree to create. The lifecycle owns naming
// policy (Dir and Branch); the adapter owns the git plumbing.
type CreateOpts struct {
	ProjectPath string // registered Project path; may be a subdirectory of its repo
	Dir         string // absolute directory to create the worktree at
	Branch      string // branch created at Base for this worktree
	Base        string // ref to branch from; empty means HEAD
}

// Created reports what a successful Create produced.
type Created struct {
	// CWD is the Session's working directory inside the worktree: the worktree
	// root, or the directory matching the Project's subdirectory of its repo
	// for monorepo Projects.
	CWD string
}

// RemoveOpts describes one worktree removal.
type RemoveOpts struct {
	ProjectPath string // used to reach the parent repo even when Dir is already gone
	Dir         string
	Force       bool // remove even when the worktree is dirty
	// DeleteBranch, when non-empty, also deletes that branch. Only spawn
	// rollback sets it (the branch was created moments ago and points at its
	// base); rm/prune never delete branches.
	DeleteBranch string
}
