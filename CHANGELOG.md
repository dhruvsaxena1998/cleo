# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

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
