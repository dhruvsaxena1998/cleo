# cleo Backlog

Living document of features, fixes, and refactors that have been considered but deferred. Each entry should have enough context that we don't have to re-research the problem when picking it up.

When an item is taken into a release, move it from this file into the corresponding spec under `docs/superpowers/specs/`.

---

## Deferred from v0.1 → candidates for v0.2 / v0.3

### v0.2 (planned — see `2026-05-09-v02-design.md` once written)

Already scoped into the v0.2 brainstorm. Listed here only for traceability; the spec is the source of truth.

### Implemented since this backlog was written

#### `tmux pane alive but agent process dead` detection

- Implemented in PR #45.
- This is no longer an active v0.3 backlog item. Reopen only if similar stale-running-session bugs appear.

#### Real hook plugins for opencode and pi

- Implemented before this backlog cleanup.
- Pi and OpenCode now have hook integrations installed by `cleo init` and checked by `cleo doctor`.

### v0.3 candidates still deferred

#### `FindByCwd` returns `multi_match_first` reason

- Today the cwd lookup returns one session ID; the handler can't tell if there were multiple candidates.
- Extend `FindByCwd` to return `(sid string, multi bool, err error)` and have the handler set `fallback_reason = "multi_match_first"` when `multi` is true.
- Small change but invasive (interface + all call sites + tests). Defer to v0.3.

#### `cleo logs` aggregator

- Combines `hook-trace.log`, `hook-errors.log`, and (optionally) per-session event logs into a single chronological view.
- v0.2 covers per-session events via `cleo events <session-id>` and partial trace surfacing in `cleo doctor`. The aggregator is mainly useful when something cross-session is wrong — relatively rare.
- Likely flags: `--since`, `--errors-only`, `--follow`, `--session <id>`.

#### Structured JSON logging mode

- `CLEO_LOG_FORMAT=json` (or similar) makes cleo's own diagnostic output machine-parseable.
- v0.2 users who want machine output can use `cleo events --json` for per-session data; this is the broader "all of cleo's logs" version.
- Useful only once cleo is part of someone's larger automation stack — premature for now.

#### `cleo trace` synthetic-test command

- Spawns a known-good test session, fires synthetic hook events, verifies attribution end-to-end, then tears down.
- Was in v0.2 Approach B. v0.2 doctor's attribution-failure summary catches most regressions retroactively, which is cheaper.
- Worth reconsidering if doctor's retroactive view turns out to miss real problems.

#### ANSI passthrough in pane preview

- Today `tmux capture-pane -p` strips escape sequences; the preview shows text without colors. With `-e` we get raw escape sequences but lipgloss renders them as literal characters.
- Fix needs a lightweight ANSI parser that converts the escape stream into lipgloss styled segments while clamping width. Non-trivial but not huge.
- High visual win — agent UIs (claude in particular) lean heavily on color for state cues.

#### Multi-session preview cache

- Currently single-buffer (designed in v0.2). If users complain about preview flicker on selection change, cache the last capture per visible session and replace on tick.
- Trade-off is memory footprint; for ≤ 10 sessions per project the cost is negligible.

#### Reconciler state-machine refactor

- Today `WaitingForInput → Completed` is a two-hop via `Idle` (with the v0.2 fix unblocking it). Cleaner: single-hop with a separate `WaitingForInputTimeout` config knob, defaulting to the same value as `IdleToCompletedTimeout`.
- Keeps the state machine more honest at the cost of one more config knob.

#### Per-session sound mute in TUI

- v0.1 already has cleo-wide sound toggle on `m`. Per-session mute (e.g. `M` to mute the highlighted session) would let users silence one noisy agent without globally going dark.
- Needs a new field on the session struct or a separate mute-list file.

---

## Deferred from v0.1 ship session (May 2026)

### Goreleaser `brews` → `homebrew_casks` migration

- `brews` is deprecated in goreleaser v2; will be removed in a future major. Casks are the new canonical formula publisher even for non-cask formulae.
- Low urgency until the deprecation actually fires.

### Windows sound support

- `internal/sound/player.go` currently has macOS (`afplay`) and Linux (`paplay`/`aplay`/`play`) branches; Windows falls through to `Available() = false`.
- Native Windows option: `powershell -c "(New-Object Media.SoundPlayer '<file>').PlaySync()"` for WAV; or PowerShell + `System.Speech` for ding tones.
- Goreleaser archives are darwin/linux only too; would need to add `windows/amd64` builds.

### Branch protection on `main`

- For solo dev with a Codex/Claude PR loop this is mostly noise, but once external contributors arrive: require PR, dismiss stale reviews, status checks pass.
- Set via repo settings or the `.github/CODEOWNERS` + branch-protection-rules config.

### Issue templates beyond bug-report

- v0.2 will add a basic bug-report template. Feature-request, question, and "support" templates are nice-to-have.

### README screencast / GIF

- A 30s asciicast or GIF of `cleo init → cleo run claude → switch to TUI → see hook events flow` would do more for adoption than any prose.
- Capture: `asciinema rec` then `agg` to convert to GIF, or use `vhs` for scripted recording.
