# cleo — design spec

**Status:** approved (brainstorming complete; awaiting implementation plan)
**Date:** 2026-05-07
**Owner:** dhruvsaxena1998

---

## 1. TL;DR

`cleo` is a TUI-first terminal session manager for AI coding agents (Claude Code, Codex, opencode, pi). Each agent runs as its own tmux session. Cleo observes state via the agents' native hook systems and renders a project-grouped dashboard so the user can keep ambient awareness across many concurrent agent sessions without context-switching between them.

The core invariant: agents persist in tmux. Cleo is observability and orchestration; it can be closed and reopened freely without disrupting any running work.

## 2. Goals

- See state for every running agent at a glance (running / waiting-for-input / idle / error / completed).
- Spawn, attach, kill, and rename agent sessions from a TUI or matching CLI.
- Group sessions by project (registered directory) for visual organization.
- Get audible feedback when agents change state — especially when an agent needs user input.
- Keep agents running and tracked even when the cleo TUI is closed.
- Single static binary, easy install, zero background services for v0.1.

## 3. Non-goals (v0.1)

- Pane-tail state detection for agents without hooks (opencode, pi spawn fine, but show perpetual `running` state).
- Inline config editor in the TUI — users edit `config.toml` directly.
- Adopting agent sessions started outside cleo.
- Initial-prompt input in the spawn popup.
- Web dashboard, remote agents, multi-machine coordination.
- Windows.

## 4. Architecture

```
                      ┌─ ~/.claude/settings.json
                      │   (hook entries pointing at `cleo hook ...`)
                      ▼
┌──────────────┐   hook events   ┌────────────────────────┐
│ Claude Code  │ ──────────────▶ │  cleo hook (shim)      │
│ Codex        │                 │   • append events.jsonl │
└──────────────┘                 │   • update state.json   │
        │ runs inside            │   • play sound          │
        ▼                        └─────────┬──────────────┘
   ┌─────────┐                             │
   │  tmux   │◀── cleo spawns/attaches ────┤
   │ session │                             │
   └─────────┘                             │
                                           ▼
                       ~/.config/cleo/
                       ├── config.toml          (user-edited)
                       ├── projects.json        (registered projects)
                       ├── state.json           (live session snapshot)
                       └── events/<sid>.jsonl   (per-session event log)
                                           │
                                           ▼
                       ┌──────────────────────┐
                       │  cleo  (TUI / CLI)   │
                       │  reads state files,  │
                       │  drives tmux         │
                       └──────────────────────┘
```

**Key choice:** no daemon. Hooks fired by Claude / Codex are the event bus. The cleo binary itself is the hook handler (dispatched by subcommand), so there is no separate executable to ship. The TUI is purely a reader/driver — it can be closed and reopened without losing observability, because the hooks keep writing state and playing sounds independently.

A future daemon (for remote agents, web UI, cross-machine state) can be slotted in later without changing the on-disk file formats. The events log + state file already form a clean migration interface.

### 4.1 Lifecycle

1. **One-time setup:** `cleo init` writes hook entries into `~/.claude/settings.json` (and Codex's equivalent) that invoke `cleo hook <event>`. Detects pre-existing entries on these hook keys and prompts user to merge / overwrite / abort.
2. **Project registration:** `cleo add [path]` (default cwd) writes a project entry into `projects.json`.
3. **Spawn:** user picks an agent in the TUI (or runs `cleo run <agent>`). Cleo executes:
   ```
   tmux new-session -d -s <session-id> -c <project-path> -e CLEO_SESSION_ID=<session-id> '<agent-command>'
   ```
4. **Run:** agent works. Hooks fire on PreToolUse / PostToolUse / Notification / Stop / SessionStart / SessionEnd. The shim reads the hook's stdin JSON + `CLEO_SESSION_ID` env, derives a state transition, atomically updates `state.json`, appends to the per-session events log, and (for sound-mapped events) launches the configured sound player non-blocking.
5. **Observe:** the TUI watches `state.json` via fsnotify and tails event logs. Closed TUI is fine — hooks keep writing.
6. **Reconcile on launch:** the source of truth for "what is actually running" is `tmux ls` filtered by `cleo-*` prefix. Sessions in `state.json` not in tmux are marked `dead`. Cleo-prefixed tmux sessions not in `state.json` are surfaced for adoption (deferred to v0.2; v0.1 logs and ignores them).

## 5. Data model

### 5.1 Project

```jsonc
// ~/.config/cleo/projects.json
{
  "projects": [
    {
      "id": "myapp",                          // slug from path basename, kebab-cased, deduped
      "name": "myapp",                        // display name (user-editable)
      "path": "/Users/dhruv/Dev/myapp",
      "default_agent": "claude",              // optional override of [defaults] default_agent
      "added_at": "2026-05-07T10:00:00Z"
    }
  ]
}
```

`cleo run` resolves the project by walking up from `pwd` until a registered project root matches. No match → cleo prompts: `register /path as a new project? [Y/n]` (auto-register with confirmation).

### 5.2 Session

```jsonc
// ~/.config/cleo/state.json
{
  "version": 1,
  "sessions": {
    "cleo-myapp-claude-fix-auth-bug": {
      "id": "cleo-myapp-claude-fix-auth-bug",   // == tmux session name
      "project_id": "myapp",
      "agent": "claude",
      "name": "fix-auth-bug",                   // user-given (or counter fallback)
      "state": "waiting_for_input",
      "started_at": "...",
      "last_event_at": "...",
      "last_message": "Allow Bash command 'rm -rf node_modules'?",
      "tool_count": 17
    }
  }
}
```

**Session ID format:** `cleo-<project-id>-<agent>-<slug>`.

- If the user provides a name in the spawn popup: `<slug>` = name slugified, case-folded, deduped against existing sessions in this `(project, agent)` pair (collisions get `-2`, `-3`).
- If the user leaves the name empty: `<slug>` = `<n>` where `n` is a monotonic counter per `(project, agent)` pair, never reused. Example: `cleo-myapp-claude-3`. The popup placeholder shows `claude-3` for clarity, but the agent isn't doubled in the tmux session name because the path already encodes it.

### 5.3 State machine

```
   spawning ──▶ running ⇄ waiting_for_input
                    │
                    ▼
                  idle ──▶ completed
                    │
                    ▼
                  error
                    │
                    ▼
                   dead   (tmux session gone — set by reconciler; remains in state.json until `cleo prune`)
```

| State                | Meaning                                                                   |
|----------------------|---------------------------------------------------------------------------|
| `spawning`           | tmux session created, first hook event not yet received                   |
| `running`            | a tool is currently executing (PreToolUse received without matching Stop) |
| `waiting_for_input`  | Notification hook fired (agent is asking for permission/input)            |
| `idle`               | Stop event fired; agent finished its turn                                 |
| `completed`          | SessionEnd received, or `idle` with no resumption within timeout          |
| `error`              | non-zero exit detected, or error pattern in event payload                 |
| `dead`               | tmux session no longer exists; entry stays in state.json until pruned     |

### 5.4 File layout

```
~/.config/cleo/
├── config.toml                    user-edited
├── projects.json                  registered projects
├── state.json                     live session snapshot
├── state.json.lock                advisory lock for concurrent writes
├── events/
│   ├── cleo-myapp-claude-fix-auth-bug.jsonl
│   └── archive/
│       └── cleo-myapp-claude-old-session.jsonl.gz
├── sounds/                        bundled WAVs extracted on first run
│   ├── start.wav
│   ├── attention.wav
│   ├── done.wav
│   └── error.wav
└── hook-errors.log                shim failure log (best-effort)
```

## 6. Hook integration

### 6.1 Claude Code → cleo state mapping

| Hook event       | cleo transition                                   | Sound event         |
|------------------|---------------------------------------------------|---------------------|
| `SessionStart`   | `spawning → running`                              | `session_start`     |
| `PreToolUse`     | if not already `running`, transition to `running`; otherwise no-op | (silent) |
| `PostToolUse`    | stay `running`                                    | (silent)            |
| `Notification`   | `running → waiting_for_input`                     | `needs_input`       |
| `Stop`           | `running → idle`                                  | `session_idle`      |
| `SubagentStop`   | logged; no top-level transition                   | (silent)            |
| `SessionEnd`     | any → `completed`                                 | `session_completed` |

`idle → completed`: if `SessionEnd` doesn't arrive but the session sits `idle` past a configurable timeout (default 10 min), the reconciler promotes it to `completed`. Set `[retention] idle_to_completed_timeout` to tune.

### 6.2 Codex → cleo state mapping

Same conceptual transitions. Exact Codex hook event names to be confirmed during implementation against the current Codex CLI release. The `[agents.codex] hooks = "codex"` config tells the shim which protocol to apply.

### 6.3 Correlation

When cleo spawns an agent, it sets `CLEO_SESSION_ID=<session-id>` in the tmux session env via `tmux new-session -e`. tmux propagates env to the agent process, which inherits it. When a hook fires, `cleo hook <event>` reads `os.Getenv("CLEO_SESSION_ID")` from its inherited env. That is the link.

If `CLEO_SESSION_ID` is empty (user ran the agent outside cleo) → shim is a no-op and exits 0. Cleo never sees that session.

### 6.4 Concurrent writes

Multiple hook invocations across different sessions can fire concurrently.

- Acquire `flock(2)` advisory lock on `state.json.lock` for the read-modify-write window.
- Write to `state.json.tmp` then `rename(2)` (atomic on POSIX).
- Hook shim must complete in <50ms typical. Sound playback is fire-and-forget (`exec.Command(...).Start()`, never `.Wait()`). The hook's shell timeout in claude/codex configs is set to 2s as a backstop.

### 6.5 Hook shim failure mode

If `cleo hook` panics or returns non-zero:
- Claude/Codex treat hook failure as advisory; the agent continues.
- The shim writes a single line to `~/.config/cleo/hook-errors.log` (best-effort, not locked).
- State may go briefly stale; the reconciler on next TUI launch corrects it from `tmux ls` + the events log.

## 7. Configuration

`~/.config/cleo/config.toml` is created with documented defaults on first run. Schema:

```toml
# Global defaults
[defaults]
detach_key      = "C-b d"          # tmux native; shown in TUI footer when attached
default_agent   = "claude"

# Sound subsystem
[sound]
enabled = true
volume  = 0.7                      # 0.0–1.0; passed to player as -v on macOS

[sound.events]
session_start     = "start.wav"
needs_input       = "attention.wav"
session_idle      = "done.wav"
session_completed = "done.wav"
session_error     = "error.wav"
# Empty string disables that event. Paths can be absolute, ~-relative,
# or bare filenames (resolved against ~/.config/cleo/sounds/, then the
# bundled defaults inside the binary).

# Agent definitions — drives the spawn popup and the bracket label/color.
[agents.claude]
command = "claude"
label   = "cl"
color   = "#CC785C"
hooks   = "claude"                 # "claude" | "codex" | "none"

[agents.codex]
command = "codex"
label   = "cx"
color   = "#10A37F"
hooks   = "codex"

[agents.opencode]
command = "opencode"
label   = "oc"
color   = "#FF6B35"
hooks   = "none"                   # v0.1: shows perpetual `running`

[agents.pi]
command = "pi"
label   = "pi"
color   = "#7C3AED"
hooks   = "none"

# UI tweaks
[ui]
show_pane_preview     = true       # the live mirror in `v` view
pane_preview_lines    = 30
pane_preview_interval = "1500ms"
event_log_lines       = 200
sidebar_width         = 32

# Retention
[retention]
hint_threshold              = 6    # advisory banner threshold (finished sessions per project)
prune_keep_default          = 5    # `cleo prune --keep` default
idle_to_completed_timeout   = "10m"
```

Validation: cleo refuses to start with a parse-error config and prints the offending key/line. Unknown keys log a warning but don't fail.

## 8. CLI surface

| Command                           | Purpose                                                              |
|-----------------------------------|----------------------------------------------------------------------|
| `cleo`                            | Launch TUI                                                           |
| `cleo init`                       | One-time: install hooks into `~/.claude/settings.json` + Codex config |
| `cleo add [path]`                 | Register a project (default: cwd)                                    |
| `cleo rm <project>`               | Unregister a project (running sessions keep running, just untracked) |
| `cleo run <agent> [--name N]`     | Spawn `<agent>` in current project; auto-registers cwd with prompt   |
| `cleo ls`                         | List projects + sessions                                             |
| `cleo attach <session-id>`        | Attach to a running session (`tmux attach -t ...`)                   |
| `cleo kill <session-id>`          | Kill a session (confirms first unless `--yes`)                       |
| `cleo prune [project] [--keep N] [--all] [--dry-run]` | Remove finished sessions; archives event logs   |
| `cleo hook <event>`               | Internal — invoked by hook configs; reads stdin, updates state       |

`cleo run` flow:
1. Resolve project: walk up from cwd until a registered project root is found.
2. If none: prompt `register /path as a new project? [Y/n]`. On `n`, exit. On `Y`, register, then continue.
3. Slugify `--name` (or generate `<agent>-<n>` counter if not given).
4. Build session ID `cleo-<project>-<agent>-<slug>`. Dedupe by suffix if collision.
5. `tmux new-session -d -s <id> -c <path> -e CLEO_SESSION_ID=<id> '<command>'`.
6. Print one-line confirmation and exit. (Does not auto-attach.)

## 9. TUI

### 9.1 Layout

```
┌─ cleo ────────────────────────────────────────────────────────────────────────┐
│ Projects                 │ cleo-myapp-claude-fix-auth-bug                     │
│                          │                                                    │
│ ▼ myapp                  │ ● running    started 12m ago                       │
│   [cl] fix-auth-bug   ●  │ tools: 17    last event: 3s ago                    │
│   [cx] refactor-rts   ◐⚠ │                                                    │
│   [cl] explore-perf   ○  │ ── recent ────────────────────────────────────────│
│ ▼ shuruhq                │ 12:34:01  PreToolUse  Edit /src/api/routes.ts     │
│   [cl] migrate-db     ○  │ 12:34:03  PostToolUse Edit (ok)                   │
│   [pi] explain-arch   ●  │ 12:34:04  PreToolUse  Bash 'pnpm test'            │
│ ▶ pickup    (no agents)  │ 12:34:18  PostToolUse Bash (ok, 14.2s)            │
│ ▶ queries   (no agents)  │ 12:34:19  Stop                                    │
│                          │                                                    │
│                          │ ── pane preview ──────────────────────────────────│
│                          │ > implementing the new endpoint                   │
│                          │   ⏺ Edit src/api/routes.ts                        │
│                          │   ⏺ Bash pnpm test                                │
│                          │   ✓ all 142 tests passed                          │
│                          │                                                    │
├──────────────────────────┴────────────────────────────────────────────────────┤
│ n new  v view  ↵ attach  k kill  / filter  a add  m mute  ? help  q quit     │
└───────────────────────────────────────────────────────────────────────────────┘
```

- **State glyphs:** `●` running · `◐` waiting-for-input (with `⚠` when long-waiting) · `○` idle · `✓` completed · `✗` error · `☠` dead.
- **Agent labels:** `[cl]` `[cx]` `[oc]` `[pi]` — bracketed two-letter codes painted in agent brand color (config-driven).
- **Empty projects:** rendered collapsed, dimmed, with `(no agents)` hint.
- **Retention banner:** appears as a top-of-pane strip when any project exceeds `hint_threshold` finished sessions.

### 9.2 Keybindings

| Cursor on…           | Key       | Effect                                                         |
|----------------------|-----------|----------------------------------------------------------------|
| Project              | `n`       | Open agent-picker popup → selection spawns in this project     |
| Project              | `space`   | Collapse / expand                                              |
| Project              | `r`       | Rename project (inline edit)                                   |
| Project              | `delete`  | Unregister project (confirm)                                   |
| Agent                | `v`       | Show this agent in the main pane (events log + pane mirror)    |
| Agent                | `↵`       | Attach to tmux session (`Ctrl-b d` to detach back)             |
| Agent                | `r`       | Rename agent (inline; renames tmux session too)                |
| Agent                | `k`       | Kill agent (confirm)                                           |
| (anywhere)           | `a`       | Add project (path picker, defaults to cwd)                     |
| (anywhere)           | `/`       | Enter filter mode (substring match across project + agent name)|
| (anywhere)           | `m`       | Toggle mute (persists to config)                               |
| (anywhere)           | `?`       | Help overlay                                                   |
| (anywhere)           | `q`       | Quit cleo TUI (agents keep running)                            |

Filter mode keys: type to refine; `enter` freezes the filter; `esc` clears; `/` re-enters input.

### 9.3 Spawn popup

```
┌─ Spawn agent in 'myapp' ───────────────┐
│                                        │
│   Agent: > claude                      │
│           codex                        │
│           opencode                     │
│           pi                           │
│                                        │
│   Name:  [_________________]           │
│          ↑ optional — default: claude-3│
│                                        │
│   tab to switch field   ↵ spawn   esc  │
└────────────────────────────────────────┘
```

- Agent list comes from `[agents.*]` keys in config.
- Empty name → slug is the bare counter `<n>` (per `(project, agent)` pair, monotonic, never reused). Placeholder in the input shows `<agent>-<n>` for human readability.
- Slug is dedup'd against existing sessions in this `(project, agent)` pair; collisions get `-2`, `-3`.
- After spawn: dismiss popup, brief toast `spawned cleo-myapp-claude-fix-auth-bug — press ↵ to attach`.

### 9.4 View pane (the `v` action)

- Top: header line — session id, state, started-at, tool count, last event.
- Middle: events log (most recent `event_log_lines` from `events/<sid>.jsonl`, tailed live).
- Bottom: pane preview — last `pane_preview_lines` from `tmux capture-pane -p -t <sid>`, polled every `pane_preview_interval`.

Both sections are read-only. Use `↵` to attach for interaction.

## 10. Sound

| Event             | Default sound       | Fires when…                                                |
|-------------------|---------------------|------------------------------------------------------------|
| `session_start`   | `start.wav`         | First hook event after spawn                               |
| `needs_input`     | `attention.wav`     | `running → waiting_for_input` transition                   |
| `session_idle`    | `done.wav`          | `running → idle` transition                                |
| `session_completed` | `done.wav`        | any → `completed`                                          |
| `session_error`   | `error.wav`         | any → `error`                                              |

PreToolUse / PostToolUse are silent by design — too noisy.

**Players (probed once at startup):**
- macOS: `afplay -v <volume> <file>`
- Linux: `paplay <file>` → fallback `aplay <file>` → fallback `play <file>` (sox)
- Windows: not supported in v0.1

If no player is found: log warning once, run muted. No errors.

**Bundled sounds:** four short (<300 ms) royalty-free WAVs `embed`-ed in the Go binary, extracted to `~/.config/cleo/sounds/` on first run if not present. Users can replace files in place; config defaults resolve to those paths.

**Mute toggle:** `m` in TUI flips `[sound] enabled`; persisted to config; toast confirms.

**Concurrency:** sound playback is `exec.Command(...).Start()` and never waited on. Hook shim returns within tens of milliseconds regardless of audio I/O.

## 11. Search & retention

### 11.1 Filter

Activated by `/`. Substring match (case-insensitive) against project name, agent slug, and session name. Empty filter restores full view. Filter is in-memory and per-TUI-instance; not persisted.

### 11.2 Retention

Sessions are never auto-deleted. Cleo *advises*.

- TUI shows a banner when any project has more than `hint_threshold` (default 6) finished sessions:
  > `'myapp' has 12 finished sessions — run 'cleo prune myapp' to clean up`
- `cleo prune` removes finished sessions (`completed` / `error` / `dead`) only. Running / waiting / idle sessions are never touched.
- "Remove" means: entry deleted from `state.json`; per-session events log gzip'd and moved to `events/archive/<sid>.jsonl.gz`. No information is destroyed.
- Variants: `--keep N` (default 5) keeps most recent N finished per project; `--all` runs across all projects; `--dry-run` previews.

## 12. Failure modes & recovery

| Failure                                                    | Behavior                                                              |
|------------------------------------------------------------|-----------------------------------------------------------------------|
| tmux not installed                                         | Hard error on first launch with platform install instructions          |
| Sound player binary missing                                | Log once, run muted                                                    |
| `~/.claude/settings.json` already has hooks for these events| `cleo init` detects, prints diff, prompts merge / overwrite / abort   |
| `cleo` binary moved (path in hook config stale)            | Hooks fail silently → state stale → reconciler on next TUI launch repairs; toast suggests `cleo init` |
| Concurrent hook writes to `state.json`                     | `flock` advisory lock + atomic rename; serialized                      |
| Hook shim slow / hangs                                     | Hook timeout (2s) configured in agent hook config kills the shim       |
| tmux session disappears (user `kill-session`)              | Reconciler marks `dead`, archives entry, no error                      |
| `state.json` corrupt                                       | Backup to `state.json.broken-<ts>`, rebuild from `tmux ls` + events    |
| Multiple cleo TUI instances open at once                   | All read-only on state.json; no conflict                               |
| Cleo-prefixed tmux session not in `state.json` (orphan)    | v0.1: log and ignore. v0.2: prompt to adopt.                           |

## 13. Cross-platform notes

- **macOS:** primary target. tmux + afplay both shipped or trivially installed (`brew install tmux`).
- **Linux:** supported. tmux from package manager. Sound via paplay / aplay / sox (any one suffices).
- **Windows:** not supported in v0.1. tmux story is poor; revisit only with WSL2 in scope.

## 14. Distribution

- Single static Go binary, ~10 MB.
- Homebrew tap: `brew install dhruvsaxena1998/tap/cleo`.
- `go install github.com/dhruvsaxena1998/cleo/cmd/cleo@latest`.
- GitHub Releases artifacts: `cleo-darwin-amd64`, `cleo-darwin-arm64`, `cleo-linux-amd64`, `cleo-linux-arm64`.
- No separate hook script files; hooks invoke `cleo hook <event>` and the binary dispatches by subcommand.

## 15. Testing approach

- **Unit tests:** state-machine transitions; config parser; slug + ID generation; event-log append; flock contention.
- **Integration tests:** spawn a stub agent (`bash -c 'sleep 60'`) inside a tmux session; inject hook payloads via `echo '<json>' | cleo hook <event>` with `CLEO_SESSION_ID` set; assert resulting state.
- **TUI snapshot tests:** Charmbracelet's `teatest` for golden-file comparisons of key flows (spawn popup, view pane, filter mode).
- **Reconciler tests:** seed `state.json` with sessions that don't exist in tmux, run reconciler, assert archival.

## 16. Implementation skeleton (suggested)

```
cleo/
├── cmd/cleo/main.go
├── internal/
│   ├── config/        # TOML load/save, defaults, validation
│   ├── state/         # state.json read/write, flock, atomic rename
│   ├── projects/      # projects.json
│   ├── tmux/          # exec wrappers: new-session, ls, kill, capture-pane
│   ├── hooks/         # subcommand handlers per protocol (claude, codex)
│   ├── sound/         # player probe + fire-and-forget playback
│   ├── reconcile/     # on-launch sync of state.json with tmux ls
│   ├── tui/           # bubbletea Model/Update/View; sub-components for sidebar, view pane, popup
│   └── cli/           # cobra commands
├── assets/sounds/     # embedded WAVs
└── docs/superpowers/specs/2026-05-07-cleo-design.md
```

## 17. Open questions / risks

- **Codex hook event names.** Confirmed by user that Codex has hooks; exact event keys to be locked in by reading current Codex CLI docs at implementation time. Implementation plan should call this out as a "verify before coding the codex hook handler" step.
- **Cleo binary path in hook configs.** If the user moves the binary, hooks fail. Mitigation: `cleo init` writes the binary's resolved absolute path; document that `cleo init` should be re-run after upgrading via a method that changes the install path.
- **`SessionEnd` reliability.** Claude may not always emit `SessionEnd` cleanly (e.g., kill-session). The `idle → completed` timeout (default 10 min) is the safety net.
- **Hook writes during agent crash.** If the agent crashes mid-tool, we may see PreToolUse without PostToolUse. The state machine treats this as `running` until reconciler decides otherwise. The 10-min idle timeout will eventually mark it `completed`. Acceptable for v0.1; revisit if user reports stuck states.
- **Long agent names eating sidebar width.** UI truncates with ellipsis at `sidebar_width - len("[xx] ") - state-glyph - padding`. Full name visible in `v` view header.

## 18. v0.1 scope vs deferred

**v0.1 (MVP) ships:**
- CLI: `cleo`, `cleo init`, `cleo add`, `cleo rm`, `cleo run`, `cleo ls`, `cleo attach`, `cleo kill`, `cleo prune`
- TUI: project sidebar tree, agent list, spawn popup with name field, `v` view (events + pane mirror), `/` filter mode, retention banner, all keybindings in §9.2
- Full hook integration: claude + codex
- Sound system with five mapped events, `m` mute toggle, bundled WAVs
- Reconciler on launch
- macOS + Linux

**Deferred to v0.2+:**
- Pane-tail state detection for hook-less agents (opencode, pi)
- Inline config editor in TUI
- "Adopt orphan" flow for sessions started outside cleo
- Initial-prompt input in spawn popup
- Web dashboard / remote agents (daemon mode)
- Windows support
