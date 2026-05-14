# Clickable Links in Attached Sessions — Design Spec

**Issue:** #37  
**Date:** 2026-05-14

## Problem

When an AI agent (Claude, Codex, etc.) outputs a URL inside a cleo-managed tmux session, the URL is rendered as blue-colored plain text. It looks like a link but is not clickable — terminal emulators cannot open it because tmux intercepts the interaction before the terminal can act on it.

## Root Cause

cleo creates tmux sessions via `tmux new-session -d` with no `allow-passthrough` option set. Without this option, tmux filters terminal escape sequences and blocks the outer terminal emulator from detecting and interacting with URLs rendered in the pane. Agents output plain ANSI-colored URLs (not OSC 8 hyperlinks), so the terminal emulator's URL detection mechanism is the only path to making them clickable.

## Chosen Approach

Set `allow-passthrough on` on the initial pane of every cleo-spawned tmux session immediately after creation. This is a pane-level tmux option (available since tmux 3.3a) that instructs tmux to pass terminal escape sequences through to the outer terminal emulator, enabling URL detection and cmd+click in Ghostty, WezTerm, and modern iTerm2.

## Design

### Change location

`internal/tmux/tmux.go` — inside `(*Client).NewSession`, after the `new-session` command succeeds.

### Implementation

After the existing `c.cmd(args...).CombinedOutput()` call returns without error, run a second command:

```
tmux set-option -pt <sessionName> allow-passthrough on
```

- `-p` — targets the pane (this is a pane-level option)
- `-t <sessionName>` — targets the newly created session's initial pane

### Error handling

If the tmux version predates 3.3a, this command will exit non-zero. The error is silently ignored — the session is created and functional, just without passthrough. Failing the entire spawn for an optional enhancement is not acceptable.

### Scope

Both spawn paths (`internal/cli/run.go` → `cleo run` and `internal/tui/handle_key.go` → TUI 'n' key) call `client.NewSession(...)`. Placing the fix inside `NewSession` covers both paths automatically.

## Out of Scope

- OSC 8 hyperlink injection (Approach B) — not needed for common macOS terminals
- tmux copy-mode URL bindings (Approach C) — worse UX, not pursued
- Changes to agent command invocation or output post-processing

## Compatibility

| tmux version | Behaviour |
|---|---|
| ≥ 3.3a | `allow-passthrough` set; URLs clickable in supported terminals |
| < 3.3a | set-option call fails silently; sessions work as before |

Supported terminals: Ghostty, WezTerm, iTerm2 (3.5+). Terminal.app does not support URL detection through tmux and will not benefit.
