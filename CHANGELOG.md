# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [0.2.1] - 2026-06-08

### Added
- In-app settings editor: press `,` in the dashboard to edit a curated set of settings without leaving the TUI — `default_agent`, `editor`, `theme`, `sidebar_width`, pane-preview toggle/lines/interval, `status_timeout_seconds`, `event_log_lines`, the `[timeouts]` and `[pruning]` values, sound enabled/volume, and a per-event on/off toggle for each `[sound.events.<event>]`. Changes preview live — the theme recolors and the sidebar resizes as you scroll — and are written to `config.toml` on `enter` (re-clamped to valid ranges) or discarded on `esc`. The list scrolls to fit short terminals. The `[tmux]`, `[agents]`, and `[keybinds]` tables (and per-event sound *files*) remain file-only. Rebindable via the new `settings` keybind action.
- Auto-expiring dashboard status messages: status text set via hooks now fades after `status_timeout_seconds` (default 30s) and clears automatically, so stale "thinking" or "waiting" labels don't persist indefinitely.

### Fixed
- Suppressed the `session_idle` sound when the Pi agent ends its session, avoiding an unnecessary audible notification on normal shutdown.

## [0.2.0] - 2026-06-02

### Added
- Configurable main-view keybindings via the `[keybinds]` table, with defaults resolved from config and consumed by the TUI.
- `[keybinds]` validation, conflict precedence, and reserved-key enforcement: unrecognized keys are dropped (other valid keys in the same list survive), a key claimed by two actions resolves to the higher-importance one (first-wins), and `enter`/`esc`/`ctrl+c` stay reserved as always-on attach/cancel/quit hatches that can never be reassigned or locked out.
- Boot warnings popup: config problems (keybind conflicts, unknown theme, clamped values) now surface in a ✓/✗ popup on launch in addition to `cleo doctor`.
- Dashboard editor action: open the selected project with `ctrl+g` or `e`, using `[ui].editor` first and `$EDITOR` as fallback.
- Center-screen fuzzy finder for projects and sessions, plus total agent memory in the topbar.

### Changed
- Help popup (`?`) and footer hints now derive their key labels from the resolved keymap instead of hardcoded strings, so a rebind (e.g. `kill = ["x"]`) is reflected in both. The help popup lists every key bound to an action; the footer shows each action's first key.
- User docs now live as GitHub-rendered Markdown under `docs/`, with `README.md` slimmed down to a docs hub and landing-page docs links pointed at the Markdown source of truth.
- Session lifecycle, tmux attach, hook handling, and reconciliation internals were deepened behind clearer seams for more reliable attach, kill, prune, rename, and project-session removal flows.

### Fixed
- Finder layout and highlighting now use full-width selection rows and up/down-only navigation.
- Pane preview jitter is reduced with content diffing and a 2s polling interval.
- Cleo reports missing agent commands before launch and surfaces spawn failures in the TUI footer.
- Immediately exited tmux sessions are detected sooner, and stale focus entries are pruned during reads to keep `focus.json` bounded.
- The redundant `v` view hint is hidden when pane preview is already enabled.

### Removed
- Removed the duplicated static `html/cleo/docs.html` docs page. GitHub-rendered Markdown under `docs/` is now the user-docs source of truth.

## [0.1.2] - 2026-05-25

### Added
- Send-keys popup: press `m` on a session to send text directly to its tmux pane via `send-keys`.
- Add project from TUI: the spawn popup (`n`) now includes an editable path field with filesystem autocomplete, so you can register new projects without leaving the dashboard.
- ANSI color passthrough in terminal preview: `tmux capture-pane -e` preserves escape sequences, and preview refresh interval is now 500ms for faster updates.

### Changed
- `cleo hook <protocol>` renamed to `cleo hooks invoke <protocol>` everywhere (configs, hook templates, docs).
- `cleo hooks` CLI restructured: `init`, `cleanup`, and hidden `invoke` subcommands.
- `sidebar_width` config default bumped from 32 to 48 columns.
- Mute keybinding moved from `m` to `alt+m` (now that `m` opens the send-keys popup).
- Spawn and rename inputs now replace spaces with dashes, max 32 characters.
- Popup widths increased (send-keys 68, spawn 64, help 58).

### Fixed
- `sidebar_width` config was parsed and validated but never wired to TUI rendering — sidebar was always 36% of terminal width. Now uses the configured value.

## [0.1.1] - 2026-05-23

### Changed
- Config schema reorganized around subsystem sections: top-level `default_agent`, `[tmux]`, nested `[sound.events.<event>]`, `[ui.pane_preview]`, `[timeouts]`, and `[pruning]`.
- Agent config now owns command, label, and color only; hook integration is installed and checked through the hook protocol layer.
- README and website docs now document the current config schema and Pi/OpenCode hook support.

### Fixed
- TUI session spawn failures now roll back the optimistic state entry instead of leaving a stale failed session behind.
- Backlog docs no longer list already-implemented Pi/OpenCode hooks or PR #45 tmux/agent-death handling as active v0.3 candidates.

## [0.1.0-beta.5] - 2026-05-15

### Added
- Remove a project from cleo via the new CLI command and `D` keybind in the TUI.
- Hook trace now records the sound decision (`played` / `focus` / `idle-nudge` / `disabled`) for every event, making it easier to diagnose missing notifications.
- `cleo init` is now idempotent — re-running skips hook events whose command is already wired up, instead of duplicating them.

### Fixed
- Tmux: `allow-passthrough` is set on new sessions so OSC-8 clickable URLs work inside cleo-spawned panes (#37).
- TUI: `j` / `k` keys are no longer swallowed by the spawn popup while typing into the name field.
- OpenCode plugin: corrected plugin format and the `question` tool state mapping so events register reliably.
- Focus: stale `focused=true` TTL reduced from 30 minutes to 5 minutes to limit false-positive sound suppression after window switches.

## [0.1.0-beta.4] - 2026-05-13

### Added
- OpenCode hook integration: `cleo init` installs a TypeScript plugin to `~/.config/opencode/plugins/cleo.ts`; `cleo doctor` checks plugin status and hook activity; 7 lifecycle events mapped to cleo state transitions and sound events.

## [0.1.0-beta.3] - 2026-05-12

### Added
- Heap memory usage displayed in the TUI topbar.
- `P` keybind to prune finished sessions for the focused project directly from the tree panel.
- `init` and `cleanup` CLI commands now use inline y/N prompts instead of the huh multi-select widget.

### Fixed
- Cursor navigation: viewport now scrolls to keep the cursor row visible when moving up/down through the tree.
- Cursor navigation: `cursorUp` from the first session of an expanded project correctly lands on the previous project's last session.
- Completed sessions are revived in the TUI when the underlying tmux pane is still alive.
- Terminal states (`Dead`, `Completed`, `Errored`) now ignore late-arriving hook events to prevent accidental resurrection.
- `re-attach` to completed sessions works again when the tmux pane is still alive.
- Hooks: idle-nudge suppression is now scoped to Claude `Notification` events via `SuppressWhenIdle`; other events are unaffected.
- Hooks: notification sound plays even when `state.Apply` fails.
- Hooks: CWD fallback attempted for Claude when `CLEO_SESSION_ID` is stale or unknown.
- Hooks: Claude/Codex hook timeout raised from 2s to 5s to reduce lock-contention drops.
- Config: UI fields are merged individually so a partial `ui:` block no longer wipes unrelated keys.
- Config: `sound.enabled` is now a `*bool`; an absent key defaults to `true` instead of `false`.
- Focus: stale `focused=true` entries expire after 30 minutes.
- Focus: `client-focus-out` hook registered so focus clears on tmux window switch.

### Changed
- Dropped `charmbracelet/huh` dependency; replaced with lightweight inline prompts.

## [0.1.0-beta.2] - 2026-05-11

### Added
- Unified `Protocol` interface for hook handling; `ClaudeProtocol`, `CodexProtocol`, and `PiProtocol` each implement `Normalize()` and `Install()`/`Cleanup()`.
- Pi hook support: `PiProtocol` with install/cleanup and a TypeScript extension template.
- `doctor` now checks whether the Pi extension file exists and diffs its content against the expected template.
- `init` includes Pi in the hook-installation selection and reformats output with lipgloss colors.
- `--payload` flag on `cleo hooks handle` for manual event injection during debugging.
- Tmux presence check on startup — cleo exits with a clear message if tmux is not found.
- Help keybinds panel in the TUI (`?`).
- curl one-liner installer (`install.sh`).

### Fixed
- Hook trace now populates the `event` field; nil `State` guarded in `applyNormalized`.

## [0.1.0-beta.1] - 2026-05-10

Beta on the road to v0.1.0 stable. Stability fixes, observability tooling, and TUI polish driven by friction surfaced during real cleo use. See [docs/superpowers/specs/2026-05-10-v02-design.md](docs/superpowers/specs/2026-05-10-v02-design.md) for the design rationale (the spec was authored under a "v0.2" working title before the version was reframed; the content still applies).

### Added
- Hook trace log entries now include a `fallback_reason` field documenting how the session was resolved (`env_present`, `env_missing`, `env_unknown_session`, `no_match`). Surfaced by `cleo doctor` in v0.2.
- `cleo events <session-id> [-f] [--type <kind>] [--since <duration>] [-n <limit>] [--json]` — print or tail per-session event logs. Supports substring matching and archived (`cleo prune`) sessions.
- `cleo doctor` now prints recent hook traces, an attribution-failure summary (last 24h), and a +/- diff between expected and installed hook entries. New `--quiet` flag suppresses passing checks for cron use.
- `CONTRIBUTING.md` documenting build/test commands, commit format, and manual verification rituals (pane preview correctness, reconciler timing).
- Bug-report issue template at `.github/ISSUE_TEMPLATE/bug_report.yml` (auto-applies `bug` and `triage` labels).

### Fixed
- Sessions in `waiting_for_input` now correctly progress to `completed` after two idle-timeout windows. Previously, the synthetic `idle_timeout` event bumped `last_event_at`, restarting the timer indefinitely.
- Pane preview no longer freezes after rapid navigation. Preview ticker is now selection-driven and self-recovering.
- `pane_preview_lines` is honored (was silently ignored in v0.1).
- Long captured lines are truncated to the panel width instead of wrapping and breaking the layout.
- Whitespace-only pane (e.g. `--no-attach` agent) shows an attach hint instead of an unhelpful "loading…".

### Changed
- Reconciler now annotates a spawning-timeout advance with `LastMessage = "advanced from spawning by reconciler (no startup hook seen)"`, surfaced in the TUI events panel.
- `cleo ls` honors `retention.spawning_timeout` (previously hardcoded to 30s).
- Esc has a predictable hierarchy: closes the active popup, then exits filter mode (clearing the query), then clears the status line. `q` no longer quits while inside a popup or filter.
- Status line clears on any user-initiated state change (cursor move, expand/collapse, popup open, filter entry), not just navigation.

## [0.1.0-alpha.1] - 2026-05-09

First public alpha. Terminal session manager for AI coding agents.

### Added
- TUI dashboard with project sidebar, event log, and pane mirror
- Hook-based lifecycle tracking for Claude Code and Codex
- 5 built-in themes with terminal background sync (OSC 11/111)
- CLI: `add`, `rm`, `ls` (`--json`, age column), `run`, `attach`, `kill`, `prune`, `rename`, `init`, `cleanup`, `doctor`, `focus`
- Cross-platform sound playback (macOS afplay, Linux paplay/aplay/play) with focus-aware suppression
- Persistent state at `~/.config/cleo` with archived event logs
- Goreleaser-based release pipeline with homebrew tap

### Known limitations
- opencode and pi are managed-only (no hooks plugin)
- No Windows support
