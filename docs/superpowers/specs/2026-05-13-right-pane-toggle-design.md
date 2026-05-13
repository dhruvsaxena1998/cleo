# Right Pane Toggle Design

**Date:** 2026-05-13
**Status:** Draft
**Depends on:** `2026-05-13-production-tui-and-alert-reliability-design.md`

## Overview

The right pane currently stacks the event log above the tmux preview in a fixed layout. This spec replaces that with two distinct modes — **Event log** and **Terminal preview** — toggled with `tab`. The session header strip is shared between modes and never changes.

The motivation: the event log (structured tool history) and the tmux preview (raw agent output) serve different information needs. Forcing both on screen at once shrinks each to the point of being barely useful. A toggle lets the user choose the right view for the moment without losing either.

---

## Right Pane Layout

All modes share the same session header strip at the top and the tab bar directly below it. Only the content area below the tab bar changes.

```
┌─ session header strip ─────────────────────────────────────────┐
│  ✽  ◈ cl  refactor-api  ·  working    ⚒ 28  ·  7m  enter attach│
├─ tab bar ──────────────────────────────────────────────────────┤
│  󰋙 Event log [active]      󰙊 Terminal [tab]       tab to switch│
├─ content area (switches on toggle) ────────────────────────────┤
│                                                                 │
│  (event log OR tmux preview renders here)                       │
│                                                                 │
└─────────────────────────────────────────────────────────────────┘
```

### Session header strip

Unchanged from the production TUI spec. Shows: state icon, agent badge, session name, state label, tool count, age, attach CTA. Color follows state.

### Tab bar

Two tabs: **Event log** and **Terminal**. The active tab shows `[active]`; the inactive tab shows `[tab]` to hint the toggle key. A right-aligned label `tab to switch` reinforces the keybinding. Tab bar is always visible regardless of session state.

---

## Mode 1 — Event Log

Compact timeline view. Built entirely from events already in the session's `.jsonl` event log — no tmux polling required in this mode.

### "Now" section

Shown at the top when session state is `running` or `waiting_for_input`. Derived from the most recent unmatched `PreToolUse` event (i.e., a `PreToolUse` with no subsequent `PostToolUse` for the same invocation).

```
⟳ Now                                           8s running
┌─────────────────────────────────────────────────────┐
│  ✎  Edit   src/api/routes.go                   8s ▸  │
│  ▓▓▓▓▓▓▓▓▓░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░       │
└─────────────────────────────────────────────────────┘
```

- Tool icon: `✎` Edit, `⚙` Bash, `󰈍` Read, `▶` generic
- Tool name + argument (truncated)
- Elapsed timer (seconds since `PreToolUse` event timestamp)
- Animated indeterminate progress bar

When session is `idle`, `completed`, `failed`, or `stopped`: the Now section is hidden. History starts at the top.

When session is `waiting_for_input`: Now section shows the permission request text from `last_message` instead of a tool name.

### "Recent" history section

Dot-based timeline, chronological descending (most recent at top after the Now card). Each entry is a tool invocation represented by a paired `PreToolUse` + `PostToolUse`, or an event like `SessionStart`, `Stop`, `Notification`, `SessionEnd`.

**Entry layout:**
```
● Tool name          duration
  detail / file / command
  → result (if available)
```

**Dot colors:**
- `●` green — Bash/tool completed with exit 0 or success
- `●` red — Bash/tool completed with exit non-zero or error
- `●` dim/outline — Edit, Read, Write (no exit code — neutral)
- `●` purple outline — SessionStart
- `●` blue outline — Stop / SessionEnd

**Detail line:** file path + line count for Read/Edit/Write, command for Bash.

**Result line (Bash only):** `→ exit 0 · 42 passed` or `→ exit 1 · 2 failed`. Parsed from the `message` field of the `PostToolUse` event where available; omitted if not parseable.

**SessionStart entry:** shown at the bottom in muted style. Displays `SessionStart · agent · project · N min ago`.

### Data source

Events are read from `~/.config/cleo/events/<session-id>.jsonl` — the same file the existing event log reads from. The timeline view is a different rendering of the same data. No new data collection is required.

Pairing `PreToolUse` + `PostToolUse`: match by tool name and sequential position in the log (not by a unique ID, since the event log doesn't carry one). If a `PreToolUse` has no matching `PostToolUse` (still in-flight), it becomes the "Now" card.

---

## Mode 2 — Terminal Preview

Raw tmux pane capture. Functionally identical to the existing tmux preview, relocated into the toggle framework.

- Captures output via `tmux capture-pane -t <session-id> -p -e` (existing implementation)
- Refreshes every 1.5s (configurable via `[ui].pane_preview_interval`)
- Strips ANSI escape codes for clean display
- Shows last N lines (configurable via `[ui].pane_preview_lines`, default 30)
- Polling only runs when Terminal mode is active — suspends when Event log mode is visible

Polling suspension: when the user switches to Event log mode, stop the `previewTickCmd` timer. When switching back to Terminal, restart it. This avoids unnecessary tmux subprocess spawning when the preview isn't visible.

**Config: `show_pane_preview = false`:** If the user has disabled the preview in config, the Terminal tab is hidden from the tab bar entirely. Only the Event log tab is shown and `tab` has no effect. This preserves the existing opt-out behaviour.

---

## Keyboard Mapping

| Key | Action |
|-----|--------|
| `tab` | Toggle right pane between Event log and Terminal preview |

`tab` acts on the right pane regardless of which pane currently has focus. It does not conflict with any existing keybinding. The existing `v` key (detached view) is unaffected.

### Default mode

Event log. This avoids starting tmux polling before the user needs it. Mode is not persisted across cleo restarts — always opens in Event log mode.

---

## State-Awareness

| Session state | Event log "Now" card | Terminal mode |
|---|---|---|
| `working` (running/spawning) | Current tool + timer | Live output, cursor visible |
| `needs input` (waiting_for_input) | Permission request from `last_message` | Permission dialog visible |
| `idle` | Hidden — history only | Shell prompt, agent idle |
| `completed` | Hidden — history only | Session output, shell prompt |
| `failed` | Hidden — history only | Error output visible |
| `stopped` | Hidden — history only | Tmux gone — shows "session stopped" placeholder |

When state is `stopped`, Terminal mode shows a placeholder: `○  Session stopped — tmux session is no longer available`. The tab bar still renders but the Terminal tab is visually dimmed.

---

## Out of Scope

- Scrolling the event log within the toggle view (existing scroll behaviour carries over)
- Configurable default mode per session or globally
- Exporting or copying from either mode
- Third toggle mode (e.g., diff view, file tree)

---

## Testing

**Toggle key:** Press `tab` with a session selected, verify right pane switches from Event log to Terminal. Press again, verify it switches back. Verify `tab` has no effect when no session is selected.

**Now card — running state:** While a session is in `running` state, verify the "Now" card shows the correct tool name from the last `PreToolUse` event and the elapsed timer increases each second.

**Now card — waiting state:** While a session is in `waiting_for_input`, verify the "Now" card shows the `last_message` content (permission request text) rather than a tool name.

**Now card hidden:** With a session in `idle` state, verify the Now section is absent and history begins at the top.

**History dot colors:** Trigger a Bash tool call that exits 0 — verify green dot. Trigger one that exits non-zero — verify red dot. Trigger a Read — verify dim/outline dot.

**Terminal polling suspend:** Switch to Event log mode, verify no `tmux capture-pane` subprocess is spawned (check via `hook-trace.log` or process list). Switch to Terminal mode, verify polling resumes.

**Stopped session Terminal mode:** Select a stopped session, switch to Terminal mode, verify placeholder text is shown rather than a tmux error.

**Tmux preview unchanged:** In Terminal mode, verify content matches existing tmux preview behaviour (same lines, same refresh interval).
