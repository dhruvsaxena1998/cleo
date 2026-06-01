# Cleo Glossary

A single source of truth for every term in Cleo's domain language. When writing code, specs, docs, or commit messages, use these terms exactly.

---

## Core Concepts

**Cleo**
The application itself: a terminal session manager for AI coding agents. Manages projects, spawns agent sessions in tmux, and tracks lifecycle via hooks.

**Project**
A directory registered with Cleo that agents can work in. Has an ID, name, and path. Stored in `projects.json`.
_Avoid_: workspace, repo

**Session**
A running agent instance in tmux, tied to one Project and one agent type. Has a state machine (`spawning` → `running` → `idle` → `completed`/`error`/`dead`). Stored in `state.json`. Session IDs follow the shape `cleo-<project-id>-<agent>-<session-name>`.
_Avoid_: task, job, run

**Session lifecycle**
The creation, attachment, revival, termination, pruning, and renaming flow for a Session. Coordinates durable state, tmux, focus tracking, and events. Removing a Project removes Session records and event logs for that Project's Sessions. Does not include Worktree behaviour until that feature is actively planned.
_Avoid_: session service, runner

**Worktree**
A git worktree created by Cleo for an agent session, living at `<project>/.cleo/worktrees/<session-name>/` on branch `cleo/wt-<session-slug>`. Isolates agent work from the project's main working tree so multiple agents can work in parallel without branch conflicts. Branches off the current HEAD by default, overridable with `--base <branch>`. Worktrees persist after session end and are cleaned up by `cleo prune` (or `cleo kill`). Whether a project uses worktrees by default is configurable per project in `projects.json`.
_Avoid_: sandbox, isolated workspace

**Agent**
A supported terminal-based AI coding tool that Cleo can spawn and track. Built-in agents: `claude` (Claude Code), `codex` (Codex), `pi` (Pi), `opencode` (OpenCode). Custom agents can be added via `config.toml`.
_Avoid_: provider, driver

**Hook**
A shell command injected into an agent's configuration that fires on agent lifecycle events. When triggered, the hook calls `cleo hooks invoke` with the agent name, event type, and a JSON payload. Cleo's hook handler normalizes and processes this into state transitions, event-log entries, and sound decisions.
_Avoid_: callback, trigger (when you mean the cleo-side mechanism)

**Hook event**
A named occurrence emitted by an agent (e.g. `SessionStart`, `UserPromptSubmit`, `Stop`). Each agent protocol maps its raw event names to Cleo's canonical `NormalizedEvent`.
_Avoid_: hook trigger, agent signal

**Event log**
A per-session JSONL file (`events/<session-id>.jsonl`) recording every hook event Cleo received for that session. Each entry has a timestamp, type, optional tool name, detail message, and optional duration. Pruned sessions have their event logs gzipped and moved to `events/archive/`.
_Avoid_: audit trail, history

**Dashboard**
The TUI launched by plain `cleo` (no arguments). Shows registered projects, their sessions, current state, a preview pane, and action footer.
_Avoid_: UI, app window

**Preview pane**
The right-hand panel in the TUI dashboard that shows captured tmux pane output for the selected session. Uses `tmux capture-pane -p`. Refreshed at a configurable interval (`ui.pane_preview.interval`).
_Avoid_: output pane, terminal preview

---

## Seams and Interfaces

**Tmux seam**
The single interface (`sessionlifecycle.Tmux`) through which the Session lifecycle drives tmux — spawning a session, checking liveness, binding the detach key, installing focus hooks, killing, and producing the attach invocation (`switch-client` inside tmux, `attach-session` otherwise). The real `tmux.Client` satisfies it in production; a fake satisfies it in tests. The lifecycle depends on this seam alone and never reaches past it to the concrete client.
_Avoid_: tmux launcher, tmux wrapper, tmux client (when you mean the interface — `tmux.Client` is the production adapter, not the seam)

**Agent protocol**
One supported agent integration, behind the single `hooks.Protocol` interface; `hooks.Protocols()` is the registry. It is the source of truth for everything cleo must know about an agent: the hook events it fires, the config files it owns (`Locations()`), how to install / clean up / diagnose those files, how to normalize a raw hook event, and its display identity. `init`, `cleanup`, and `doctor` iterate the registry rather than switching on agent name. The concrete structs (`ClaudeProtocol`, `CodexProtocol`, …) are the adapters; the interface is the seam.
_Avoid_: agent driver, hook plugin, agent adapter (when you mean the seam — the concrete struct is the adapter, "Agent protocol" is the interface)

**Hook outcome**
The complete set of effects a normalized hook event produces: the Session state transition, the event-log entry, and the sound decision (play, or the reason it was suppressed — disabled, focus, or idle-nudge). Computed purely by `hooks.decideHook` from the normalized event, the pre-transition state, and whether sound is enabled / the Session is focused; `applyNormalized` gathers those inputs, calls the decision, then performs the outcome. The pure decision is the test surface — no temp dirs, config, or fakes.
_Avoid_: hook result, hook action (when you mean the decision — the outcome is the data, `decideHook` is the decision)

**Normalized event** (`NormalizedEvent`)
The canonical form every protocol produces after parsing its raw payload. Contains a state event, sound event, message, tool name, and flags (`LogOnly`, `SuppressWhenIdle`). `Handle()` consumes only this — no protocol-specific logic lives outside the `Protocol` implementation.

---

## State Machine

**State**
One of seven values a Session can be in. Stored in `state.json` and rendered in the dashboard.

| State | Short label | Glyph | Meaning |
|---|---|---|---|
| `spawning` | `spawn` | `◌` | Cleo created state and is starting tmux/agent process. |
| `running` | `run` | `●` | The agent is active or has recently resumed work. |
| `waiting_for_input` | `wait` | `◑` | The agent requested input, approval, or attention. |
| `idle` | `idle` | `○` | The agent stopped its current turn but the session is still available. |
| `completed` | `done` | `✓` | The session ended cleanly or aged out from idle. |
| `error` | `err` | `✗` | Cleo recorded an error state. |
| `dead` | `dead` | `·` | The underlying tmux session is gone. |

**Finished session**
A session whose state is `completed`, `error`, or `dead`. `IsFinished()` returns true. Prune considers only finished sessions. Finished sessions block attach and rename.
_Avoid_: done, terminated, ended

**Event** (`state.Event`)
A named transition driver. Events come from two sources: hook events (emitted by agents) and synthetic events (generated by the reconciler).

| Event | Source | Meaning |
|---|---|---|
| `session_start` | Hook | Agent reports it started; moves `spawning` → `running`. |
| `pre_tool_use` | Hook | Agent is about to use a tool; resumes from `idle`/`waiting_for_input`. |
| `post_tool_use` | Hook | Agent finished using a tool; bumps `ToolCount`. |
| `notification` | Hook | Agent needs input/approval; moves to `waiting_for_input`. |
| `stop` | Hook | Agent stopped its turn; moves to `idle`. |
| `session_end` | Hook | Agent ended cleanly; moves to `completed`. |
| `user_resume` | Synthetic | Reconciler detects user re-attached to a completed session; revives to `idle`. |
| `idle_timeout` | Synthetic | Reconciler promotes long-idle sessions toward `completed`. |
| `error` | Hook | Agent or Cleo recorded an error; moves to `error`. |
| `dead` | Synthetic | Reconciler detects tmux session no longer exists. |

**State transition**
The mapping from current `State` + incoming `Event` → next `State`, defined in `state.NextState()`.

- **Hard terminal states**: `dead` and `error` are hard-sticky — only `EvDead` can change them. No other event will transition out.
- **Soft terminal state**: `completed` is soft-sticky — `EvDead` and `EvUserResume` (reconciler reviving a completed session whose tmux is still alive) can change it, others are ignored.
- **Timeout progression**: `EvIdleTimeout` on `waiting_for_input` downgrades to `idle` first (so the user sees one cycle of "idle" before it completes), then on a second cycle from `idle` it moves to `completed`.
- **Implicit resume**: `EvPreToolUse` and `EvPostToolUse` from `spawning`, `waiting_for_input`, or `idle` all move to `running` — no explicit resume event needed.

**Apply**
A state store operation that transitions a session by event and bumps `LastEventAt`. Used for real activity (hook events, user resume). Controlled via `BumpTime: true` in `reconcile.Action`.
_Avoid_: update, transition

**ApplySynthetic**
A state store operation that transitions a session by event without bumping `LastEventAt`. Used for synthetic events (`EvIdleTimeout`, `EvDead`) that represent the absence of activity rather than activity itself. Bumping `LastEventAt` for these would reset idle timers and prevent stuck sessions from progressing. Controlled via `BumpTime: false`.
_Avoid_: silent update, background transition

---

## Reconciler

**Reconciler**
The component (`internal/reconcile`) that compares Cleo's session state against live tmux sessions and synthesizes transitions for sessions whose state is out of sync. Runs on every TUI poll (750ms) and on `cleo ls`.
_Avoid_: sync loop, state checker

**Live set**
The set of tmux session names matching the `cleo-` prefix, fetched via `tmux list-sessions`. The reconciler compares Sessions from `state.json` against this set.
_Avoid_: tmux snapshot, running list

**Action** (`reconcile.Action`)
One intended state transition produced by `Decide`. Fields:
- `SessionID` — which session to transition
- `Event` — the transition event to apply
- `Message` — optional detail message to set on the session
- `BumpTime` — `true` → use `Apply` (bumps `LastEventAt`), `false` → use `ApplySynthetic`

**Decide** (`reconcile.Decide`)
The pure function that computes the set of `Actions` needed for a given session snapshot. Signature: `(sessions []Session, liveSet map[string]bool, now time.Time, opts Options) []Action`. Has zero I/O and is fully deterministic. The test surface for the reconciler.

Decision branches:
- Session not in live set and not already `dead` → `EvDead` (BumpTime: false)
- Session is `completed` but tmux is still alive → `EvUserResume` to `idle` (BumpTime: true, restarts idle clock)
- Session `spawning` past `SpawningTimeout` → `EvSessionStart` to `running` (BumpTime: true)
- Session `idle` or `waiting_for_input` past `IdleTimeout` → `EvIdleTimeout` (BumpTime: false)

**ApplyActions**
Executor that applies a set of `Actions` through a `StateStore` interface. Switches between `Apply`/`ApplySynthetic` based on `BumpTime`.

**RunOpts**
Thin wrapper: gathers data (lists sessions, fetches live set), calls `Decide`, calls `ApplyActions`. Signature: `(st StateStore, tx TmuxLs, opts Options) error`.

**TmuxLs** (interface)
Narrow consumer-side interface: `LsPrefix(prefix string) ([]string, error)`. Used by the reconciler to get the live set.

**StateStore** (interface)
Consumer-side interface in the reconcile package: `List()`, `Apply()`, `ApplySynthetic()`. The production `*state.Store` satisfies it by structural subtyping.

**Options**
Configure timeout thresholds:
- `IdleTimeout` — from `config.toml` `[timeouts].idle_to_completed_timeout` (default: `10m`)
- `SpawningTimeout` — from `config.toml` `[timeouts].spawning_timeout` (default: `30s`)

---

## CLI: Full Command Reference

### `cleo` (no arguments)
Launches the TUI dashboard. No flags. This is the primary user-facing surface.

---

### `cleo add [path]`

Registers a project directory with Cleo.

- **Arguments**: `[path]` — directory path (optional). Defaults to current working directory if omitted.
- **Max args**: 1
- **Flags**: none
- **Output**: `registered project "<ID>" at <path>`
- **Project ID**: slugified from the directory's base name. Deduplicated with numeric suffixes (`-1`, `-2`, etc.) if the slug already exists in `projects.json`.
- **Auto-registration**: `cleo run` also calls `add` automatically for unregistered directories (with confirmation prompt, skippable via `--yes`).

---

### `cleo rm <project>`

Unregisters a project from Cleo's project registry. Does **not** delete the project directory on disk.

- **Arguments**: `<project>` — project ID (required, exact 1)
- **Flags**:
  - `--force` — remove even if active (non-finished) sessions exist
  - `--yes` / `-y` — skip confirmation prompt
- **Behavior**: Resolves `<project>` by exact ID match first, then by absolute path match. If active sessions exist and `--force` is not set, refuses with an error. On success, removes all session records and event logs for that project, then removes the project from `projects.json`.
- **Confirmation**: interactive `[y/N]` prompt unless `--yes` is set.

---

### `cleo run <agent>`

Starts an agent session in tmux. This is the primary way to launch a new session.

- **Arguments**: `<agent>` — agent name (required, exact 1). Must match a configured agent key in `config.toml`'s `[agents]` section.
- **Flags**:
  - `--name <name>` — human-friendly session name. Cleo slugifies and deduplicates it against existing sessions in the same project+agent scope.
  - `--cwd <path>` — override working directory (default: current working directory)
  - `--yes` — skip auto-registration confirmation when the directory is not yet registered
  - `--no-attach` — spawn the session without attaching to it immediately. The session starts in the background.
- **Flow**:
  1. Resolve or auto-register the project for the working directory (`ErrProjectRegistrationNeeded` returned if unregistered and `--yes` not set; prompts user, retries with yes)
  2. Slugify and deduplicate session name
  3. Write `spawning` Session to `state.json` (state-first creation)
  4. Start tmux session with `CLEO_SESSION_ID` in environment
  5. Install focus hooks and bind detach key via the Tmux seam
  6. If tmux launch fails: rollback the session record
  7. If `--no-attach` is not set: attach to the new tmux session via `tmux switch-client` (inside tmux) or `tmux attach-session` (outside tmux)
- **Generated names**: If `--name` is not provided, Cleo generates a Docker-style label (adjective-noun pair, e.g. `brave-curie`).
- **Session ID shape**: `cleo-<project-id>-<agent>-<session-name>`
- **Output**: prints project registration status (if auto-registered) and the spawned session ID.

---

### `cleo ls`

Lists registered projects and known sessions.

- **Arguments**: none
- **Flags**:
  - `--json` — output as JSON array instead of tab-separated columns
- **Behavior**: Runs the reconciler first (same as TUI poll), then prints all projects and their sessions. Columns: `PROJECT`, `AGENT`, `NAME`, `STATE`, `ID`, `AGE`.
- **Sorting**: projects alphabetically by ID. Sessions within a project sorted by `LastEventAt` descending (most recently active first). Sessions with zero `LastEventAt` sort after those with a value.
- **Empty projects**: shown with `-` in all session columns.
- **Age format**: `18m`, `2h`, `3d`.

---

### `cleo attach <session-id>`

Attaches to an existing running tmux session.

- **Arguments**: `<session-id>` — full session ID (required, exact 1)
- **Flags**: none
- **Operations**:
  - Uses the Session lifecycle's `Attach` method
  - Returns an `AttachPlan` with an `Action`: `AttachBlocked` (session is finished — cannot attach), `AttachMarkedDead` (tmux session is gone — marked dead), `AttachReady` (attachable), or `AttachRevived` (completed session whose tmux is still alive — revived to idle, then attached)
  - Runs `tmux switch-client` if already inside tmux, `tmux attach-session` otherwise
  - Calls `Done()` when the user detaches, which clears focus state
- **Detach**: Use the configured tmux detach key (default `Ctrl-b d`) to detach back to the dashboard.

---

### `cleo rename <session-id> <new-name>`

Renames a session's Cleo-side label. The underlying tmux session ID is **not** changed.

- **Arguments**: `<session-id>` (required), `<new-name>` (required, exact 2)
- **Flags**: none
- **Output**: `renamed <session-id>: <old-name> → <new-name>`
- **Constraints**: finished sessions cannot be renamed. The new name is slugified the same way `--name` is on `cleo run`.

---

### `cleo kill <session-id>`

Kills a running tmux session and removes it from Cleo state.

- **Arguments**: `<session-id>` — full session ID (required, exact 1)
- **Flags**:
  - `--yes` — skip confirmation prompt
- **Behavior**: Calls `tmux kill-session` via the Session lifecycle. Removes the session record from `state.json`. Event log is NOT archived (use `prune` for that).
- **Confirmation**: interactive `[y/N]` prompt unless `--yes` is set.

---

### `cleo prune [project]`

Removes finished sessions (states: `completed`, `error`, `dead`) from active Cleo state and archives their event logs to `events/archive/` as gzipped JSONL.

- **Arguments**: `[project]` — project ID to scope to (optional). If omitted with `--all`, considers all projects.
- **Flags**:
  - `--keep <n>` — keep the newest `n` finished sessions per project (default: `pruning.keep_default` from config, which defaults to 5). Use `--keep 0` to purge all.
  - `--all` — consider sessions across all projects (required when no project argument given)
  - `--dry-run` — print session IDs that would be pruned without changing state
  - `--yes` — skip confirmation prompt
- **Archiving**: Event logs are gzipped and moved to `events/archive/<session-id>.jsonl.gz`. Original `events/<session-id>.jsonl` is deleted.
- **Confirmation**: shows count of prunable sessions and prompts `[y/N]` unless `--yes`.

---

### `cleo hooks init`

Installs Cleo hook commands into supported agent config files and extracts bundled sound assets.

- **Arguments**: none
- **Flags**:
  - `--yes` / `-y` — install all supported hook systems without prompting
  - `--force` — overwrite conflicting hook entries
  - `--agents <list>` — comma-separated list of agent names to install (e.g. `claude,codex`). If not set, installs all.
- **Installed files**:
  - Claude Code: `~/.claude/settings.json` (hook entries in `hooks` map)
  - Codex: `~/.codex/hooks.json` + `~/.codex/config.toml` (`[features].hooks = true`)
  - Pi: `~/.pi/agent/extensions/cleo.ts` (file template)
  - OpenCode: `~/.config/opencode/plugins/cleo.ts` (file template)
- **Sound assets**: extracts bundled `.wav` files into `~/.config/cleo/sounds/`
- **Idempotency**: byte-identical files are left as-is. Divergent files are rejected unless `--force` is set.
- **Hooks store absolute path**: hooks store the absolute path to the Cleo binary. Re-run after moving the binary.

---

### `cleo hooks cleanup`

Removes Cleo hook commands from supported agent config files.

- **Arguments**: none
- **Flags**:
  - `--yes` / `-y` — skip confirmation prompt
- **Behavior**: Removes cleo-owned entries from hook files. Leaves `~/.codex/config.toml` `[features].hooks` unchanged (other Codex hooks may depend on it). Files that diverged from cleo's template are left untouched (`SkippedModified`).
- **Alias**: `cleo uninstall --yes`

---

### `cleo doctor`

Checks whether Cleo hooks look correctly installed and whether hook events have recently resolved to a Cleo session.

- **Arguments**: none
- **Flags**:
  - `--quiet` — only print failures and non-empty diagnostic sections. Exits 1 if any failures found.
- **Checks performed**:
  - **Per-protocol config checks**: each protocol's `Diagnose()` method checks file presence, content validity, and freshness (stale detection)
  - **Hook trace activity**: reads `hook-trace.log` for each protocol, reports the most recent hook event and whether it resolved, and how (via `CLEO_SESSION_ID` or cwd fallback)
  - **Attribution failures**: scan for hooks that failed resolution (`fallback_reason` = `no_match` or `env_unknown_session`) in the last 24h
  - **Config warnings**: any warnings from `config.toml` loading (e.g. unknown theme name)
  - **Hook diff**: for Claude and Codex, compares on-disk hook entries against what cleo would install — reports `=` (matched), `+` (would install), `-` (conflict)
  - **Codex approval note**: reminder that Codex keeps hook approval state internally and may need manual `/hooks` approval
- **Last hook traces**: for each protocol, shows up to 3 most recent trace rows with timestamp, event, resolved session, and fallback reason

---

### `cleo events <session-id>`

Prints or tails the event log for a session.

- **Arguments**: `<session-id>` — full session ID (required, exact 1)
- **Flags**:
  - `-f` / `--follow` — tail the file. Poll-based at 500ms cadence. Reopens on inode change (survives `cleo prune` archiving).
  - `--type <kind>` — filter to one event type (e.g. `notification`, `pre_tool_use`)
  - `--since <duration>` — only events newer than `now - duration` (Go duration string, e.g. `5m`, `1h`)
  - `-n <N>` / `--limit <N>` — show only the most recent N events. Mutually exclusive with `--follow`.
  - `--json` — emit raw JSONL lines for `jq` pipelines
- **Session resolution order**:
  1. Exact match against active sessions in `state.json`
  2. Exact match against archived event files under `events/archive/`
  3. Substring match against active+archived. Errors if multiple matches.
- **Output (default)**: tab-separated columns with lipgloss styling when stdout is a TTY, plain when piped. Columns: `TIME`, `TYPE`, `MESSAGE`.
- **Empty active session**: prints `(no events yet)` to stderr, exits 0.
- **Unknown session**: exits 1 with `unknown session: <id>`.

---

### `cleo hooks invoke <protocol> <event>` (internal)

Called by agent hook scripts. Not for direct user use.

- **Arguments**: `<protocol>` — agent name (`claude`, `codex`, `pi`, `opencode`), `<event>` — hook event name
- **Input**: reads JSON payload from stdin
- **Processing**: normalizes via protocol, decides hook outcome via `decideHook`, applies state transition, logs event, writes trace, plays sound if appropriate

---

### `cleo focus <session-id> <true|false>` (internal)

Called by tmux focus hooks on attach, detach, focus-in, and focus-out. Not for direct user use.

- Updates `focus.json` with the session's current focus boolean.
- Used by sound suppression to avoid playing sounds for the currently focused session.

---

## TUI: Full Component Reference

### Architecture

The TUI is a [Bubble Tea](https://github.com/charmbracelet/bubbletea) application (`internal/tui`). Entry point: `tui.Run(c *cli.Ctx)`. It uses the alt screen.

**Model** (`tui.Model`)
The central struct holding all TUI state:

| Field | Type | Purpose |
|---|---|---|
| `ctx` | `*cli.Ctx` | CLI context (config, state store, projects store, tmux client) |
| `theme` | `Theme` | Resolved color theme (from `ui.theme` config) |
| `projects` | `[]Project` | Loaded project list (refreshed on every poll) |
| `sessions` | `[]Session` | Loaded session list (refreshed on every poll) |
| `cursor` | `cursor` | Current selection: `projectIdx` + `agentIdx` (-1 = on project row) |
| `expanded` | `map[string]bool` | Which project IDs are expanded in the tree |
| `paneCache` | `map[string]string` | Session ID → last captured tmux pane content |
| `selected` | `string` | Session ID pinned for "v" view (empty when no pinned selection) |
| `status` | `string` | Status bar message shown in the footer |
| `statusTimerID` | `int` | Auto-increments to invalidate stale status expiry timers |
| `filter` | `string` | Current filter text (empty = no filter active) |
| `mode` | `Mode` | One of: `ModeNormal`, `ModeFilter`, `ModePopup` |
| `popup` | `tea.Model` | Active popup model (nil when no popup) |
| `help` | `help.Model` | Bubble Tea help model (unused for rendering; key hints are custom) |
| `width`/`height` | `int` | Terminal dimensions from `WindowSizeMsg` |
| `err` | `error` | Unused (kept for Bubble Tea compatibility) |
| `paneCaptureInFlight` | `bool` | Prevents overlapping `capture-pane` dispatches |
| `firstStateLoaded` | `bool` | Flips to true after first `stateLoadedMsg`; triggers immediate pane capture |
| `heapAlloc` | `uint64` | Current heap allocation (updated once per tick via `runtime.ReadMemStats`); shown in topbar |

---

### Modes

- **`ModeNormal`** — default. Key presses go to navigation and action handlers. Footer shows context-sensitive key hints.
- **`ModeFilter`** — activated by `/`. Runes append to the filter string. Enter applies (exits filter mode). Backspace deletes. Esc clears filter and exits.
- **`ModePopup`** — a modal overlay is active (spawn, rename, confirm, help, send). Key presses forwarded to the popup model. Esc dismisses the popup.

**Esc hierarchy** (spec §2.2): popup → filter → status. Esc first tries to close the active popup; if none, clears filter; if no filter, clears the status message.

---

### Layout: Frame Structure

The TUI is divided into three vertical zones:

1. **Topbar** (1 row) — application header
2. **Body** (flexible) — split horizontally into left column and right column
3. **Footer** (1 row) — context-sensitive key hints and status messages

Background colour is stamped on every line so no transparent gaps show the terminal default.

```
┌─────────────────────────────────────────────────────┐
│ cleo  ai agents    3 projects  2 live  15.2 MB  muted │  ← topbar
├──────────────────────┬──────────────────────────────┤
│ ┌ Filter panel ┐     │ ┌ Session metadata (6-row) ┐ │
│ │ / myapp      │     │ │ agent  state  project... │ │
│ └──────────────┘     │ └─────────────────────────┘ │
│ ┌ Tree panel ──┐     │ ┌ Events panel ───────────┐ │
│ │ ▾ myapp    2 │     │ │ 14:05:23  session_start │ │  ← body
│ │   [cx] fix   │     │ │ 14:05:25  pre_tool_use  │ │
│ │ ▸ otherproj  │     │ └─────────────────────────┘ │
│ └──────────────┘     │ ┌ Terminal Preview ────────┐ │
│                      │ │ > Editing auth.go       │ │
│                      │ └─────────────────────────┘ │
├──────────────────────┴──────────────────────────────┤
│  ↵ attach  r rename  K kill  n new  / filter  q quit │  ← footer
└─────────────────────────────────────────────────────┘
```

---

### Layout: Left Column

#### Filter Panel (fixed height: 6 rows)

Rendered via `renderFilterPanel(w, h)`.

- **Inactive state**: shows dimmed hint text: `/ type to filter sessions and projects`
- **Active state** (filter mode): shows `/` in gold bold + current filter text + `▌` cursor, e.g. `/ myapp▌`
- **Applied filter**: when filter is set but mode is normal, shows `/` + filter text in bold, e.g. `/ myapp`
- **Hint**: displays `inactive` / `active` / or the filter text itself in the hint position

#### Tree Panel (flexible height)

Rendered via `renderTreePanel(w, h)`. Shows projects and their sessions in a tree view.

**Project rows** (collapsed): `▸ <project-id>` + session count badge on the right
**Project rows** (expanded): `▾ <project-id>` + session count badge on the right
**Selected project row**: full-row highlight with arrow indicator

**Session rows** (indented, per expanded project):
```
├ [cl] fix-auth-bug         run  2m ago
└ [cx] refactor-auth        idle 18m ago
```
- Tree connectors: `├` for non-last, `└` for last session in project
- Agent badge: `[label]` in the agent's configured color (e.g. `[cl]` in Claude orange)
- Session name: truncated to fit available width
- State: 4-char abbreviated label (`run`, `wait`, `idle`, `spawn`, `done`, `err`, `dead`) in the state's color
- Age: relative time since last event (or started-at if no events)

**Selected session row**: full-row highlight with bold text

**Session count badge colors**:
- Green when at least one active session exists
- Subtext0 (dimmed) when all sessions are finished

**Scrolling**: the tree scrolls to keep the cursor visible. `cursorFlatIdx()` computes the cursor's 0-based row position accounting for expanded/collapsed state.

**Sorting**: projects alphabetically by ID. Sessions within a project sorted by `StartedAt` ascending (earliest first), with `ID` as tiebreaker.

#### Actions Panel (at bottom of left column, not always rendered)

Rendered via `renderActionsPanel(w, h)`. Context-sensitive action list:

**When a session is selected**:
- `› attach tmux session` (primary, gold marker)
- `spawn sibling agent`
- `view session detail`
- `kill after confirm`

**When no session is selected (project row or empty)**:
- `› spawn new agent` (primary)
- `filter sessions`
- `expand / collapse`
- `quit`

---

### Layout: Right Column

#### Session Metadata Panel (fixed height: 6 rows)

Rendered via `renderMetaPanel(w, h, sess, has)`.

**When no session selected**: shows dimmed "no session selected" message.

**When session selected**: 6-cell grid with label row and value row:

| Agent | State | Project | Runtime | Tools | Last |
|---|---|---|---|---|---|
| `[cl] claude` | `waiting_for_input` | `myapp` | `2h ago` | `14` | `30s` |

- **Agent**: badge + dimmed agent name
- **State**: bold text in the state's color
- **Project**: truncated project ID in bold
- **Runtime**: relative time since `StartedAt` (with "ago" suffix)
- **Tools**: `ToolCount` from session
- **Last**: relative time since `LastEventAt` (human duration)

**Hint**: truncated session ID in the panel title area.

#### Events Panel (~28% height)

Rendered via `renderEventsPanel(w, h, sess, has)`.

- Shows the last N event log entries for the selected session, where N = `contentH` (panel height minus 4 border rows).
- Entry format: `14:05:23  session_start     my message                        1.2s`
  - Timestamp in `15:04:05` format
  - Event type in event-type color (16-char column)
  - Detail message (truncated to fit)
  - Duration (if > 0, right-aligned)
- **Event type colors**:
  - `PreToolUse` / `pre_tool_use` → Peach
  - `PostToolUse` / `post_tool_use` → Green
  - `Stop`, `SessionEnd`, `idle_timeout` → Peach
  - `Notification`, `user_resume` → Accent
  - `SessionStart` → Accent
  - `error`, `dead` → Red
  - Others → Subtext0
- **Notification detail** is rendered in Gold (attention-grabbing).
- **No session selected**: shows "select a session to view events"

#### Terminal Preview Panel (remainder of right column)

Rendered via `renderPreviewPanel(w, h, sess, has)`.

- **No session selected**: "navigate to a session to view its terminal"
- **Finished session**: "tmux session is gone; press K to remove this record"
- **Empty cache**: "loading… press v to refresh"
- **Blank capture**: "agent hasn't rendered yet — press Enter to attach"
- **Active content**: shows bottom N lines of captured tmux pane output (N = panel height minus 4 border rows). ANSI escape sequences preserved for agent output colors.
  - Trailing blank lines stripped (full-screen TUIs pad to terminal height)
  - Lines truncated with ANSI-awareness (`ansi.Truncate`) to prevent mid-escape-sequence slicing
- **Hint**: `tmux capture-pane -p` in the panel title

---

### Navigation

**Cursor** is a `cursor` struct with:
- `projectIdx` — index into the current visible project list
- `agentIdx` — session index within the expanded project; `-1` means "on the project row"

**Arrow keys / vim-style**:
- `up` / `k` → move cursor up (`cursorUp()`). From a session row: move to previous session, or to the project row if at the first session. From a project row: move to previous project; if it's expanded, jump to its last session.
- `down` / `j` → move cursor down (`cursorDown()`). From a project row: move to first session if expanded, else to next project. From a session row: move to next session, or to next project if at the last session.
- `space` → toggle expand/collapse on the current project.

**Clamp cursor**: after every navigation, filter change, or state load, `clampCursor()` ensures the cursor indices are within bounds of the current visible tree.

---

### Key Bindings (ModeNormal)

Full keymap from `DefaultKeymap()`:

| Key | Binding | Action |
|---|---|---|
| `up` / `k` | `↑/k` | Move cursor up |
| `down` / `j` | `↓/j` | Move cursor down |
| `enter` | `↵` | Attach to selected session (if on a session row; no-op on project row) |
| `n` | `n` | Open spawn popup to create a new session |
| `v` | `v` | View selected session (capture pane, pin to right panel). Hidden when preview pane auto-refresh is enabled. |
| `K` / `ctrl+k` | `K` | Confirm-kill popup for the selected session |
| `P` | `P` | Confirm-prune popup for the focused project |
| `r` | `r` | Open rename popup for the selected session (finished sessions blocked) |
| `D` | `D` | Confirm-remove-project popup |
| `m` | `m` | Open send popup (send text to session's tmux pane) |
| `alt+m` | `alt+m` | Toggle sound mute |
| `/` | `/` | Enter filter mode |
| `?` | `?` | Open help popup |
| `esc` | `esc` | Dismiss popup → clear filter → clear status (hierarchy) |
| `q` | `q` | Quit the dashboard |
| `space` | `space` | Expand/collapse the selected project |

---

### Popups

All popups are Bubble Tea models rendered as an overlay on top of the main frame. The overlay is center-positioned with `renderOverlay()`.

#### Spawn Popup (`popup_spawn`)

Triggered by `n`. Collects:
- **Agent**: select from dropdown of configured agents
- **Name**: text input, slugified on submit
- **Path**: project path (pre-filled from cursor project or CWD)
- **Project**: if path is not a registered project, option to register

On submit (`SpawnSubmitted`), calls `lifecycle.Create()` and reloads state via `loadStateCmd`. On cancel (`SpawnCancelled`), clears popup.

#### Rename Popup (`popup_rename`)

Triggered by `r` on a session row. Pre-filled with the current session name. On submit (`RenameSubmitted`), calls `lifecycle.Rename()` and reloads state. On cancel (`RenameCancelled`), clears popup.

#### Confirm Popup (`popup_confirm`)

Triggered by `K` (kill), `P` (prune), `D` (remove project). Shows a confirmation prompt with the action description.

Three confirmation kinds:
- `confirmKindKill` — on yes (`ConfirmYes`), calls `lifecycle.Kill()`
- `confirmKindPrune` — on yes, calls `lifecycle.Prune()` with `Keep: 0` (unlimited removal from TUI context, since it's project-scoped)
- `confirmKindRemoveProject` — on yes, calls `lifecycle.RemoveProjectSessions()` then removes the project from the projects store

On no (`ConfirmNo`), clears popup.

#### Help Popup (`popup_help`)

Triggered by `?`. Shows the full keybinding reference with the configured detach key. On close (`HelpClosed`), clears popup.

#### Send Popup (`popup_send`)

Triggered by `m` on a live session. Text input to send keystrokes to the session's tmux pane. On submit (`SendSubmitted`), calls `tmux.SendKeys()`. Finished sessions and dead sessions are blocked.

---

### Polling and Refresh

**State poll** (`tickStateCmd`):
- Fires a `tickStateMsg` every 750ms
- On receipt: loads state via `loadStateCmd` (which runs the reconciler, lists projects, lists sessions), fires another `tickStateCmd` for the next cycle
- Updates `heapAlloc` for the memory display in the topbar

**Preview tick** (`previewTickCmd`):
- Fires a `previewTickMsg` at the configured interval (`ui.pane_preview.interval`, default 1.5s)
- On receipt: if preview pane is enabled and a live session is selected and no capture is in flight, dispatches `capturePaneCmd` and schedules the next tick
- Skips captures for finished sessions
- Prevents overlapping captures via `paneCaptureInFlight` flag
- Content diffing: `paneCache` is only updated if content changed, avoiding unnecessary re-renders

**First-load capture**: the very first `stateLoadedMsg` triggers an immediate `autoCaptureCmd` so the preview renders within ~tmux-ls latency instead of waiting for the first preview tick (up to 1.5s of "loading…").

**Status expiry**: status messages auto-clear after 3 seconds via `statusExpiryCmd`. Each status message gets a monotonic `statusTimerID`; only the most recent timer's expiry is honored.

**Auto-expand on first load**: projects with sessions are auto-expanded when first discovered.

---

### Filtering

Activated by `/`. The `filter` string is matched case-insensitively against:
- Project IDs
- Session IDs
- Session names
- Agent names

**Matching logic** (`matchesFilter`): a project is visible if its ID matches OR at least one of its sessions matches. Sessions within a visible project are individually filtered.

**Cursor clamping**: `clampCursor()` is called after every filter change to keep the cursor within the visible tree.

**Enter**: applies the filter and exits filter mode. **Esc**: clears the filter and exits.

---

### Footer

Context-sensitive key hints rendered by `renderFooter(width)`.

**When status is set and not filtering**: shows the status message in red bold + `esc clear` + `q quit`.

**When in filter mode**: shows `enter apply` + `esc clear` + "type to filter projects and sessions".

**Default mode with live session selected**: `↵ attach`, `r rename`, `K kill`, `n new sibling`, `space collapse`, `/ filter`, `m send`, `q quit`. If preview pane is disabled, also shows `v view`.

**Default mode with finished session selected**: shows session state status + `K remove`, `P prune project`, `n new sibling`, `/ filter`, `q quit`.

**Default mode on project row (no session selected)**: `n new session`, `space expand`, `j/k move`, `↵ attach`, `D remove project`, `/ filter`, `m send`, `q quit`. If the project has finished sessions, also shows `P prune`.

---

### Topbar

Rendered by `renderTopbar(width)`.

- **Left**: `cleo` (in mauve bold) + `ai agents` (dimmed)
- **Right**: four pills + sound status:
  - `N projects` (Subtext0)
  - `N live` (Green) — sessions in `running`, `spawning`, or `idle`
  - `N waiting` (Peach) — sessions in `waiting_for_input`
  - `N.N MB` (Overlay0) — heap allocation
  - `sound on` / `muted`

---

### Pruning Banner

If any project has more finished sessions than `pruning.hint_threshold` (default 6), a gold banner is rendered: `hint  myapp has 8 finished sessions  run: cleo prune myapp`.

---

### Fallback Views (used by `renderMain` and snapshot tests)

**Empty state** (no sessions): shows "No sessions yet" with instructions to register projects and spawn agents.

**Dashboard overview** (sessions exist but none selected): shows session summary counts + a table of all sessions with columns: agent badge, session ID (truncated), state glyph + text, started-at age.

**Session detail** (single session selected via `v`): shows full session metadata, last 9 event log entries, and up to 12 lines of preview pane content. Uses `SectionDivider` between sections.

---

### Themes

Resolved via `Resolve(themeName string) Theme` from config `ui.theme`. Unknown values fall back to `catppuccin-mocha` and are reported as config warnings.

**`Theme` struct fields**: `Base`, `Mantle`, `Crust`, `Surf0`, `Surf1`, `Overlay0`, `Subtext0`, `Text`, `Accent`, `Gold`, `Green`, `Blue`, `Peach`, `Mauve`, `Red`, `Yellow`

**Built-in themes**: `catppuccin-mocha` (default), `gruvbox-dark`, `onedark`, `void`, `synthwave`

**Theme rendering methods**:
- `StateColor(s string)` — maps state names to theme colors
- `StyledGlyph(s string)` / `StyledStateText(s string)` — colored state indicators
- `AgentBadge(label, bgColor string)` — colored badge with agent label on agent color background
- `Pill(label string, fg Color)` — pill-shaped label with colored foreground on Mantle background
- `KeyHint(k, desc string)` — gold bold key + dimmed description
- `PanelBox(title, hint string, body []string, w, h int)` — box-drawing panel with title bar, separator, and body content
- `SectionDivider(label string, width int)` — horizontal rule with label
- `EventTypeColor(evType string)` — maps event types to theme colors
- `FormatEventRow(e Entry, width int, highlight bool)` — renders one event log entry as a formatted row

**Terminal background sync**: on startup, `Run()` emits OSC 11 to set the terminal background to the theme's `Base` color, preventing gaps from showing the terminal's configured background. OSC 111 is emitted on exit to restore it.

---

## Sound and Focus

**Sound event**
A named event type that triggers audio playback. Mapped in `[sound.events]`. Five events: `session_start`, `needs_input`, `session_idle`, `session_completed`, `session_error`.
_Avoid_: alert, notification sound

**Sound suppression**
Cleo suppresses session sounds while that exact tmux session is focused. This prevents duplicate attention sounds when you are already attached to the agent and watching it work. Suppression reasons:
- `disabled` — sound is globally disabled (`sound.enabled = false`)
- `focus` — the session is currently focused (user is attached and watching)
- `idle-nudge` — the protocol's event is an idle-nudge and the session is already `idle` (no point re-alerting)

**Sound players**: `afplay` on macOS (with `-v` volume flag), first available of `paplay`/`aplay`/`play` on Linux. Volume is only applied on macOS.

**Focus tracking**
Cleo installs tmux `focus-events` and client focus hooks into spawned sessions. Those hooks call `cleo focus <session-id> true/false` on attach, detach, focus-in, and focus-out. State is stored in `focus.json` with a 5-minute TTL to self-heal crash scenarios. Best-effort: if Cleo cannot determine focus, it plays sounds rather than risk hiding an alert.
_Avoid_: attention tracking, active session tracking

---

## Configuration

**config.toml**
Cleo's TOML config file at `~/.config/cleo/config.toml` (or `$XDG_CONFIG_HOME/cleo/config.toml`). Created on first run with defaults. Partial configs are supported: unspecified keys are filled from defaults at load time.

**Config sections:**

| Section | Key | Default | Purpose |
|---|---|---|---|
| (top-level) | `default_agent` | `"claude"` | Default agent for flows that need one |
| `[tmux]` | `detach_key` | `"C-b d"` | Tmux detach key for spawned sessions |
| `[sound]` | `enabled` | `true` | Master sound toggle |
| `[sound]` | `volume` | `0.7` | Playback volume (macOS only, 0.0-1.0) |
| `[sound.events.session_start]` | `file` | `"start.wav"` | Sound file for session start |
| `[sound.events.session_start]` | `enabled` | `true` | Whether to play on session start |
| `[sound.events.needs_input]` | `file` | `"attention.wav"` | Sound file for input requests |
| `[sound.events.needs_input]` | `enabled` | `true` | Whether to play on input requests |
| `[sound.events.session_idle]` | `file` | `"done.wav"` | Sound file for idle |
| `[sound.events.session_idle]` | `enabled` | `true` | Whether to play on idle |
| `[sound.events.session_completed]` | `file` | `"done.wav"` | Sound file for completion |
| `[sound.events.session_completed]` | `enabled` | `true` | Whether to play on completion |
| `[sound.events.session_error]` | `file` | `"error.wav"` | Sound file for errors |
| `[sound.events.session_error]` | `enabled` | `true` | Whether to play on errors |
| `[agents.<name>]` | `command` | — | Executable command Cleo starts inside tmux |
| `[agents.<name>]` | `label` | — | Short 2-3 char label for TUI badges |
| `[agents.<name>]` | `color` | — | Hex color for TUI badges |
| `[ui]` | `theme` | `"catppuccin-mocha"` | Color theme name |
| `[ui]` | `sidebar_width` | `48` | Sidebar width in character columns (10-200) |
| `[ui]` | `event_log_lines` | `200` | Max event log entries to tail |
| `[ui.pane_preview]` | `enabled` | `true` | Show tmux pane previews |
| `[ui.pane_preview]` | `lines` | `30` | Number of tmux pane lines to capture |
| `[ui.pane_preview]` | `interval` | `"1.5s"` | Preview refresh interval |
| `[timeouts]` | `idle_to_completed_timeout` | `"10m"` | Reconciler idle→completed timeout |
| `[timeouts]` | `spawning_timeout` | `"30s"` | Reconciler spawning→running timeout |
| `[pruning]` | `hint_threshold` | `6` | Show cleanup hint above this many finished sessions |
| `[pruning]` | `keep_default` | `5` | Default `--keep` value for `cleo prune` |

---

## Files Cleo Manages

All paths relative to `~/.config/cleo/` (or `$XDG_CONFIG_HOME/cleo/`).

| Path | Purpose |
|---|---|
| `config.toml` | User configuration |
| `projects.json` | Registered projects (JSON array of `Project` structs) |
| `state.json` | Current known sessions (JSON: `{version, sessions: map[id]Session}`) |
| `state.json.lock` | State file lock (file-based, via `flock`) |
| `focus.json` | Best-effort tmux session focus state (JSON: `{sessions: map[id]{focused, updated_at}}`). 5-min TTL self-pruning. |
| `events/<session-id>.jsonl` | Per-session event log (JSONL, one `Entry` per line) |
| `events/archive/` | Gzipped archived event logs from pruned sessions (`<session-id>.jsonl.gz`) |
| `sounds/` | Sound assets (`.wav` files) used by `[sound.events]` |
| `hook-errors.log` | Hook handler errors |
| `hook-trace.log` | Hook attribution trace used by `cleo doctor` (JSONL: `hookTraceRow` per line) |

**Agent hook files (installed by `cleo hooks init`):**

| Path | Purpose |
|---|---|
| `~/.claude/settings.json` | Claude Code hooks in `hooks` map |
| `~/.codex/hooks.json` | Codex hooks |
| `~/.codex/config.toml` | Codex hooks feature flag: `[features].hooks = true` |
| `~/.pi/agent/extensions/cleo.ts` | Pi extension (file template) |
| `~/.config/opencode/plugins/cleo.ts` | OpenCode plugin (file template) |

---

## Hook Protocol Details

**Protocol** (`hooks.Protocol`)
The Go interface every supported agent must implement:

| Method | Returns | Purpose |
|---|---|---|
| `Name()` | `string` | Internal identifier: `"claude"`, `"codex"`, `"pi"`, `"opencode"` |
| `DisplayName()` | `string` | Human-facing name: `"Claude Code"`, `"Codex"`, etc. |
| `Events()` | `[]string` | Hook event names this protocol subscribes to |
| `Locations()` | `[]Location` | Config files this protocol owns (label + absolute path) |
| `Install(cleoBin, force)` | `(InstallReport, error)` | Write hook config into agent files |
| `Cleanup()` | `(CleanupOutcome, error)` | Remove cleo-owned hook entries |
| `Diagnose()` | `[]Check` | Self-diagnosis: check file presence, content validity, freshness |
| `Normalize(event, payload)` | `(NormalizedEvent, bool)` | Convert raw event+JSON into canonical form |
| `UsesCwdFallback()` | `bool` | `true` when protocol may not propagate `CLEO_SESSION_ID` |

**Concrete protocols**: `ClaudeProtocol`, `CodexProtocol`, `PiProtocol`, `OpenCodeProtocol`

**Protocol registry**: `Protocols()` returns all registered protocols in registration order. Adding an agent means implementing `Protocol` and adding one line to `Protocols()`.

**Agent hook events (per protocol)**:

| Protocol | Hook events |
|---|---|
| Claude Code | `SessionStart`, `UserPromptSubmit`, `PreToolUse`, `PostToolUse`, `Notification`, `Stop`, `SessionEnd`, `SubagentStop` |
| Codex | `SessionStart`, `UserPromptSubmit`, `PreToolUse`, `PostToolUse`, `PermissionRequest`, `Stop` |
| Pi | `session_start`, `input`, `tool_call`, `tool_result`, `agent_end`, `session_shutdown` |
| OpenCode | `session.created`, `tool.execute.before`, `tool.execute.after`, `permission.asked`, `session.idle`, `session.deleted`, `session.error` |

**Location** (`hooks.Location`)
Contains `Label` (e.g. `"hooks"`, `"feature flag"`, `"extension"`, `"plugin"`) and `Path` (absolute).

**InstallReport**
Returned by `Install()`. Contains `ManualReview *ReviewStep` — non-nil only for Codex (gates hooks behind in-app `/hooks` approval).

**ReviewStep**
Contains `Command` (the cleo command whose entries must be approved) and `Hooks` (event names awaiting approval).

**Check** (`hooks.Check`)
One line of self-diagnosis: `Label`, `OK` (bool), `Detail` (human description). Returned by `Diagnose()`.

**CleanupStatus**
Categorises a per-protocol cleanup outcome:
- `CleanupStatusMissing` — nothing cleo-owned existed; no-op
- `CleanupStatusRemoved` — cleo content existed and was removed
- `CleanupStatusSkippedModified` — file exists but diverged from cleo's template; left untouched

**Hook trace** (`hook-trace.log`)
JSONL file recording every `cleo hooks invoke` call. Each row (`hookTraceRow`): `at`, `protocol`, `event`, `cwd`, `env_session`, `resolved_session`, `result` (resolved / no_match / env_unknown_session), `fallback_reason`. Used by `cleo doctor` for attribution analysis and activity checks.

**Cwd fallback**: when `CLEO_SESSION_ID` is not propagated by the protocol (`UsesCwdFallback() == true`), Cleo falls back to matching by working directory, selecting the most recently started active session for the matching project and agent.

---

## Identity and Naming

**Slug**
A URL-safe, lowercase, hyphenated version of a name. Generated by `ids.Slugify()`. Used for project IDs and session name components.
_Avoid_: sanitize, normalize

**DedupeSlug**
A slug guaranteed unique within a set. Appends `-1`, `-2`, etc. if the bare slug already exists. Used for project IDs via `ids.DedupeSlug`.
_Avoid_: uniquify

**Session name**
A human-friendly label for a session. Passed via `--name` on `cleo run`. If omitted, Cleo generates a Docker-style label (adjective-noun pair, e.g. `brave-curie`). The name is slugified when forming the session ID.
_Avoid_: label, task name, run name

**Session ID**
The full unique identifier for a session: `cleo-<project-id>-<agent>-<session-name>`. Used by `cleo attach`, `cleo kill`, `cleo rename`, etc.
_Avoid_: tmux session name (the underlying tmux session name happens to match the Cleo session ID, but they are conceptually distinct)

---

## Internal Packages

| Package | Key exports | Purpose |
|---|---|---|
| `cmd/cleo` | `main()` | Entry point. Delegates to `cli.Execute(tui.Run)`. |
| `internal/cli` | `Ctx`, `Execute`, all `new*Cmd` functions | All CLI commands, context setup, tmux adapter (`TmuxClient`) |
| `internal/tui` | `Model`, `Run`, `Theme`, `Resolve` | Bubble Tea TUI: model, sidebar, main pane, view, keybindings, popups, themes, styles |
| `internal/hooks` | `Protocol`, `Protocols()`, `ProtocolByName()`, `NormalizedEvent`, `decideHook`, `handler` | Hook protocol interface, 4 protocol impls, pure decision logic, handler |
| `internal/sessionlifecycle` | `Tmux` (interface), `Create`, `Attach`, `Kill`, `Prune`, `Rename`, `RemoveProjectSessions` | Session lifecycle module; depends on Tmux seam |
| `internal/reconcile` | `Decide`, `ApplyActions`, `RunOpts`, `Action`, `TmuxLs`, `StateStore`, `Options` | Reconciler: pure decision function + executors |
| `internal/state` | `State`, `Event`, `Session`, `Store`, `NextState()` | Session state machine: states, events, transitions, file-locked store |
| `internal/projects` | `Project`, `Store`, `ErrNotFound` | Project registration, resolution (`ResolveFromCwd`), file-backed store |
| `internal/events` | `Entry`, `Log`, `Archive`, `ReadOpts` | Event log append, tail, filtered read, gzip archive |
| `internal/focus` | `Store` (focus state with TTL) | Tmux focus state store with 5-min TTL self-healing |
| `internal/config` | `Config`, `Agent`, `Load`, `Save` | TOML config loading, defaults, merging, saving |
| `internal/tmux` | `Client` (implements `sessionlifecycle.Tmux` and `reconcile.TmuxLs`) | Raw tmux command execution |
| `internal/sound` | Sound player abstraction | Platform-specific playback via afplay/paplay/aplay/play |
| `internal/paths` | `ConfigRoot()` | XDG-aware config directory resolution |
| `internal/ids` | `Slugify`, `DedupeSlug`, name generation | Slug and name generation utilities |

---

## Environment Variables

| Variable | Purpose |
|---|---|
| `XDG_CONFIG_HOME` | Overrides the config root. When set, Cleo uses `$XDG_CONFIG_HOME/cleo/` instead of `~/.config/cleo/`. |
| `CLEO_SESSION_ID` | Set automatically by `cleo run` inside the spawned tmux session. Hook handlers read this to attribute events to the correct Cleo session. |
| `TMUX` | Standard tmux variable. Cleo checks it to detect that you are already inside tmux when running `cleo run`, so it can `switch-client` rather than nest a new server. |

---

## Conventions

- **Terminal states**: `dead` and `error` are hard terminal — only `EvDead` can change them. `completed` is soft terminal — can be revived by `EvUserResume`.
- **State-first creation**: A `spawning` Session is written before starting tmux. If tmux launch fails, the session is rolled back. If the process dies after write but before launch, the reconciler can mark it `dead`.
- **Best-effort focus**: Focus tracking via tmux hooks is best-effort. If Cleo cannot determine focus, it plays sounds rather than risk hiding an alert.
- **Pure decisions**: `decideHook` and `reconcile.Decide` are pure functions with zero I/O. They are the test surfaces for hook processing and reconciliation respectively.
- **Shallow seam pattern**: Consumers declare the interface they need (e.g. `sessionlifecycle.Tmux`, `reconcile.TmuxLs`). Producers (e.g. `tmux.Client`) satisfy it by structural subtyping. Interfaces are not exported from the producer package.
- **Esc hierarchy**: popup → filter → status. Esc always dismisses the innermost layer first.
- **No-attach mode**: Sessions can be spawned detached (`--no-attach`). No focus tracking is installed until the user attaches.
