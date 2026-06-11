# Cleo

Terminal session manager for AI coding agents. Manages projects, spawns agent sessions in tmux, and tracks lifecycle via hooks.

## Language

Full terminology reference: [docs/glossary.md](docs/glossary.md)

**Project**:
A directory registered with Cleo that agents can work in. Has an ID, name, and path.
_Avoid_: workspace, repo

**Session**:
A running agent instance in tmux, tied to one project and one agent type. Has a state machine (spawning → running → idle → completed/error/dead).
_Avoid_: task, job, run

**Session ID**:
The fully-qualified, unique identity of a Session: `cleo-<project>-<agent>-<name>` (e.g. `cleo-pickup-api-claude-lucid-yonath`). Serves as both the state-store key and the tmux session name. This is the string every session command (`attach`, `kill`, `rm`, `rename`, `events`) currently requires as its `<session-id>` argument.
_Avoid_: session name (the ID is the identity; the name is only the trailing slug)

**Session name**:
The human-facing slug at the tail of a Session ID — a Docker-style `adjective-noun` (e.g. `lucid-yonath`) or a `--name`-supplied custom slug. Deduplicated only within its (project, agent) scope, so a bare name is not guaranteed unique across all Sessions.
_Avoid_: session ID, label, handle

**Session lifecycle**:
The creation, attachment, revival, termination, pruning, and renaming flow for a Session. Coordinates durable state, tmux, focus tracking, and events. Removing a Project removes Session records and event logs for that Project's Sessions. Includes Worktree behaviour: creation at spawn for worktree-enabled Sessions, and removal together with the Session record. Project removal is all-or-nothing — if any of the Project's Sessions has a dirty Worktree, the removal aborts upfront (listing them) rather than half-completing; force overrides.
_Avoid_: session service, runner

**Worktree**:
A git worktree created by Cleo for an agent session, living at `<project>/.cleo/worktrees/<agent>-<session-name>/` on branch `cleo/wt-<agent>-<session-name>` (e.g. `cleo/wt-claude-lucid-yonath`). Keyed by `<agent>-<name>` — the Session ID minus the project prefix — because a bare Session name is only unique within its (project, agent) scope. Isolates agent work from the project's main working tree so multiple agents can work in parallel without branch conflicts. Cleo keeps `.cleo/` out of the parent repo's status by idempotently adding it to the repo-local `.git/info/exclude` at creation (never touching tracked files). Branches off the current HEAD by default, overridable with `--base <branch>`. Worktree creation requires a git repo with at least one commit and a resolvable base — otherwise the spawn fails before any tmux session is created (no silent fallback to the main tree). When the Project path is a subdirectory of its repo (monorepo package), the Worktree is still a full-repo worktree at the Project's `.cleo/worktrees/` location, and the Session's working directory is the corresponding subdirectory inside it. A Worktree lives exactly as long as its Session record: it survives session end and `cleo kill` (so killed work can be inspected post-mortem), and is removed when the record is removed (`cleo rm`, `cleo prune`). Removal only deletes *clean* worktrees — a dirty Worktree is skipped with a warning unless forced, and skipping the Worktree also keeps the Session record (the two live and die together) — and never deletes the branch, so committed work stays reachable. Whether a project uses worktrees by default is configurable per project in `projects.json`.
_Avoid_: sandbox, isolated workspace

**Tmux seam**:
The single interface (`sessionlifecycle.Tmux`) through which the Session lifecycle drives tmux — spawning a session, checking liveness, binding the detach key, installing focus hooks, killing, and producing the attach invocation (switch-client inside tmux, attach-session otherwise). The real `tmux.Client` satisfies it in production; a fake satisfies it in tests. The lifecycle depends on this seam alone and never reaches past it to the concrete client.
_Avoid_: tmux launcher, tmux wrapper, tmux client (when you mean the interface — `tmux.Client` is the production adapter, not the seam)

**Hook outcome**:
The complete set of effects a normalized hook event produces: the Session state transition, the event-log entry, and the sound decision (play, or the reason it was suppressed — disabled, focus, or idle-nudge). Computed purely by `hooks.decideHook` from the normalized event, the pre-transition state, and whether sound is enabled / the Session is focused; `applyNormalized` gathers those inputs, calls the decision, then performs the outcome. The pure decision is the test surface — no temp dirs, config, or fakes.
_Avoid_: hook result, hook action (when you mean the decision — the outcome is the data, `decideHook` is the decision)

**Agent protocol**:
One supported agent integration, behind the single `hooks.Protocol` interface; `hooks.Protocols()` is the registry. It is the source of truth for everything cleo must know about an agent: the hook events it fires, the config files it owns (`Locations()`), how to install / clean up / diagnose those files, how to normalize a raw hook event, and its display identity. `init`, `cleanup`, and `doctor` iterate the registry rather than switching on agent name, so adding an agent means implementing `Protocol` and adding one line to `Protocols()` — not editing every CLI command. The seam stays deep by keeping verbs thin and returns rich: `Install` reports any manual-approval follow-up (`InstallReport`), `Diagnose` returns per-agent health `Check`s, and heterogeneous extras (Codex's feature flag and approval step, the JSON-hook vs. file-template split) live inside implementations as data, never as new interface methods. The concrete structs (`ClaudeProtocol`, `CodexProtocol`, …) are the adapters; the interface is the seam.
_Avoid_: agent driver, hook plugin, agent adapter (when you mean the seam — the concrete struct is the adapter, "Agent protocol" is the interface)
