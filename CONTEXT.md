# Cleo

Terminal session manager for AI coding agents. Manages projects, spawns agent sessions in tmux, and tracks lifecycle via hooks.

## Language

**Project**:
A directory registered with Cleo that agents can work in. Has an ID, name, and path.
_Avoid_: workspace, repo

**Session**:
A running agent instance in tmux, tied to one project and one agent type. Has a state machine (spawning → running → idle → completed/error/dead).
_Avoid_: task, job, run

**Worktree**:
A git worktree created by Cleo for an agent session, living at `<project>/.cleo/worktrees/<session-name>/` on branch `cleo/wt-<session-slug>`. Isolates agent work from the project's main working tree so multiple agents can work in parallel without branch conflicts. Branches off the current HEAD by default, overridable with `--base <branch>`. Worktrees persist after session end and are cleaned up by `cleo prune` (or `cleo kill`). Whether a project uses worktrees by default is configurable per project in `projects.json`.
_Avoid_: sandbox, isolated workspace
