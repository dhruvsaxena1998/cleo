# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added
- Hook trace log entries now include a `fallback_reason` field documenting how the session was resolved (`env_present`, `env_missing`, `env_unknown_session`, `no_match`). Surfaced by `cleo doctor` in v0.2.
- `cleo events <session-id> [-f] [--type <kind>] [--since <duration>] [-n <limit>] [--json]` — print or tail per-session event logs. Supports substring matching and archived (`cleo prune`) sessions.
- `cleo doctor` now prints recent hook traces, an attribution-failure summary (last 24h), and a +/- diff between expected and installed hook entries. New `--quiet` flag suppresses passing checks for cron use.

### Fixed
- Sessions in `waiting_for_input` now correctly progress to `completed` after two idle-timeout windows. Previously, the synthetic `idle_timeout` event bumped `last_event_at`, restarting the timer indefinitely.
- Pane preview no longer freezes after rapid navigation. Preview ticker is now selection-driven and self-recovering.
- `pane_preview_lines` is honored (was silently ignored in v0.1).
- Long captured lines are truncated to the panel width instead of wrapping and breaking the layout.
- Whitespace-only pane (e.g. `--no-attach` agent) shows an attach hint instead of an unhelpful "loading…".

### Changed
- Reconciler now annotates a spawning-timeout advance with `LastMessage = "advanced from spawning by reconciler (no startup hook seen)"`, surfaced in the TUI events panel.
- `cleo ls` honors `retention.spawning_timeout` (previously hardcoded to 30s).

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
