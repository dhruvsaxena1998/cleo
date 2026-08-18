---
status: accepted
---

# Worktrees live in the project and die with the Session record

A Worktree is created at `<project>/.cleo/worktrees/<agent>-<name>/` on branch `cleo/wt-<agent>-<name>`, hidden from the parent repo by an idempotent `.cleo/` line in `.git/info/exclude`, and is removed only when the Session *record* is removed (`cleo rm`, `cleo prune`) — never by `cleo kill`. Removal deletes only clean worktrees (dirty ones are skipped with a warning unless forced, and the skip keeps the record too) and never deletes the branch.

## Why

- **Record-tied lifetime, not process-tied.** `kill` is exactly when you most want to inspect what the agent was doing; deleting the worktree then would destroy uncommitted work at the worst moment. Binding worktree and record together gives one invariant — they live and die as a unit — so there is never an orphan worktree with no record or a record pointing at nothing Cleo created.
- **`<agent>-<name>` keying.** Session names are deduplicated only within (project, agent) scope, so a bare name can collide across agents on one project. `<agent>-<name>` is the Session ID minus the redundant project prefix: unique within the project by construction, without changing existing naming rules.
- **`.git/info/exclude` over `.gitignore`.** Repo-local, touches no tracked file, needs no commit, never appears in anyone's diff — worktree support requires zero setup and leaves zero footprint in the project's history.
- **Clean-only destruction, branch kept.** Mirrors git's own `git worktree remove` safety model; committed work stays reachable on `cleo/wt-*` branches the user deletes at leisure.

## Considered and rejected

- **Worktrees outside the repo** (`~/.config/cleo/worktrees/...`) — no ignore problem, but hides agent work far from the project and breaks tooling that assumes proximity to the repo.
- **Cleanup on `kill`** — saves a step when you kill-and-forget, but destroys uncommitted work precisely when post-mortem matters.
- **Project-wide session-name dedup** instead of `<agent>-<name>` keying — fixes the collision but changes Session-name semantics for all sessions, worktree or not.
- **Deleting merged branches at prune** — less branch litter, but requires prune to know the base branch and do merge analysis; deferred until the litter is a demonstrated problem.

## Consequences

- Monorepo projects (Project path below the repo root) still get a full-repo worktree at the Project's `.cleo/worktrees/` location; the Session's working directory is the corresponding subdirectory inside it. The `.cleo/` exclude pattern matches at any depth, so the same line covers this case.
- Worktree creation fails the spawn (before any tmux session exists) on non-git projects, unborn HEAD, or an unresolvable `--base` — no silent fallback to the main working tree.
- The Session record persists the worktree path and branch so removal and TUI display never re-derive them.
- Project removal becomes all-or-nothing: any dirty worktree among the project's sessions aborts the whole removal upfront (force overrides), because half-removing would leave either orphan worktrees or records without a project.
