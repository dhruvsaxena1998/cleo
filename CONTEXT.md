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

**Session lifecycle**:
The creation, attachment, revival, termination, pruning, and renaming flow for a Session. Coordinates durable state, tmux, focus tracking, and events. Removing a Project removes Session records and event logs for that Project's Sessions. Does not include Worktree behaviour until that feature is actively planned.
_Avoid_: session service, runner

**Worktree**:
A git worktree created by Cleo for an agent session, living at `<project>/.cleo/worktrees/<session-name>/` on branch `cleo/wt-<session-slug>`. Isolates agent work from the project's main working tree so multiple agents can work in parallel without branch conflicts. Branches off the current HEAD by default, overridable with `--base <branch>`. Worktrees persist after session end and are cleaned up by `cleo prune` (or `cleo kill`). Whether a project uses worktrees by default is configurable per project in `projects.json`.
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

**Dashboard**:
The interactive local TUI opened by `cleo` with no arguments — the single place to watch every Project and Session, see each one's live state, and act on them (attach, send, rename, kill, prune). Local and interactive by definition.
_Avoid_: web dashboard, remote view (those name the read-only web page — a different thing)

**Remote view**:
The read-only web page served by the opt-in `cleo serve` command and reached over the local network by scanning a QR code. Lists every Session with its Project, agent, name, state, and age so you can tell from a phone which Session needs you. It is read-only — it never attaches to a Session or sends it input — and exists to surface Session state where the Dashboard cannot follow (a phone screen). Distinct from the Dashboard, which is the interactive local TUI.
_Avoid_: web dashboard, mobile dashboard, remote control (it is read-only, not a control surface)
