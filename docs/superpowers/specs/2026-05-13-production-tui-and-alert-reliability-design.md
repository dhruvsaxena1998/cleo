# Production TUI & Alert Reliability Design

**Date:** 2026-05-13
**Status:** Draft

## Overview

Two compounding problems make cleo feel unpolished after Claude launched their own agents view:

1. **Alert unreliability** — sounds play twice, or not at all, intermittently. Trust in notifications is broken.
2. **TUI visual poverty** — state information is present but hard to read at a glance. Project structure is preserved but session urgency is invisible.

This spec addresses both. The tmux preview pane is out of scope (separate discussion).

---

## Problem 1 — Alert Unreliability

### Observed symptoms

- Sound plays twice for the same event
- Sound sometimes doesn't play at all for `needs input` / `idle` transitions
- No visible log of why a sound was or wasn't played

### Root causes (suspected)

**Double-play:** `cleo init` run more than once installs duplicate hook entries in `~/.claude/settings.json`. Each hook entry fires independently, triggering two `cleo hook` invocations for the same event — two sounds.

**Silent misses:** `focus.json` has a 30-minute TTL. If cleo crashes or the tmux focus hook misfires, the session stays "focused" in `focus.json` for up to 30 minutes, suppressing all sounds during that window.

**Idle-nudge over-suppression:** The `SuppressWhenIdle` heuristic that prevents double-sound on Claude's internal 60s re-notification is too broad in some edge cases.

### Fix

**Deduplication at install time:** `cleo init` and `cleo cleanup` must check for existing hook entries before adding new ones. If the exact command string is already present in the hook array, skip it. This prevents double-play at the source.

**Focus TTL tightening:** Reduce focus.json TTL from 30 minutes to 5 minutes. 5 minutes is long enough to survive normal tmux detach/reattach cycles while limiting the false-positive suppression window.

**Hook-trace improvement:** The existing `hook-trace.log` already records attribution. Extend it to also log the sound decision: which sound event fired, whether it was suppressed and why (`focus`, `idle-nudge`, `disabled`, `played`). This makes intermittent issues debuggable without needing to reproduce them.

**No changes to the state machine or event flow.** This is purely a dedup + logging fix.

---

## Problem 2 — TUI Visual Design

### Goals

- Keep the project tree structure (cleo is multi-project; flat state grouping doesn't fit)
- Make session urgency unmissable without expanding every project
- Sessions sorted by urgency within each project
- State taxonomy that matches how users actually think about sessions
- Agent badge configurable via config (nerd font icon or label)
- All non-stopped sessions remain attachable

---

## State Taxonomy

Six display states replace the current seven internal states. The mapping is:

| Display state | Internal state(s) | Color | Attachable |
|---|---|---|---|
| **⚠ needs input** | `waiting_for_input` | Yellow `#f9e2af` | Yes — highest priority |
| **✽ working** | `running`, `spawning` | Blue `#89b4fa` animated | Yes |
| **∙ idle** | `idle` | Dimmed `#585b70` | Yes |
| **✓ completed** | `completed` + tmux alive | Green `#a6e3a1` | Yes |
| **✗ failed** | `error` + tmux alive | Red `#f38ba8` | Yes |
| **○ stopped** | `dead`, or `completed`/`error` + tmux dead | Grey `#6c7086` | No — prune only |

### Key design decisions

**Yellow for needs input, not red.** Red is reserved for failure. Yellow conveys "your attention is needed" without implying something broke.

**`completed` and `failed` are not terminal.** A SessionEnd or error event does not kill the tmux session. The reconciler checks tmux liveness after these events. If tmux is alive → keep `completed`/`failed` display. When tmux subsequently disappears → transition to `stopped`.

**`stopped` is the only truly terminal state.** It covers: explicit `dead` (reconciler detected tmux gone), `completed`/`error` after tmux dies, and sessions terminated via Ctrl+C, `/exit`, or `/quit`. `stopped` sessions cannot be attached; they can only be pruned with `P`.

**Stopped detection nuance:** Cleo cannot currently distinguish a clean exit from a crash at the OS level — both result in tmux disappearing. The reconciler will route both to `stopped`. Distinguishing clean vs crash exit is deferred.

### Sort order within each project

Sessions within a project are sorted by urgency, not creation time:

1. needs input
2. working
3. idle
4. completed
5. failed
6. stopped

---

## TUI Design

### Session row

Each session row renders in a single line:

```
[state-icon] [agent-badge] [name............] [last-message.....] [⚒ N] [age]
```

- **state-icon** — `⚠ ✽ ∙ ✓ ✗ ○` in state color; `✽` animates (opacity pulse)
- **agent-badge** — shows `icon` from config if set, otherwise `label`; styled with agent `color`
- **name** — truncated to ~88px, color follows state
- **last-message** — truncated `last_message` from state.json; dimmed for idle/done states; italic for needs input
- **⚒ N** — `tool_count` from state.json
- **age** — time since `last_event_at`; color follows state

Visual intensity follows urgency: `needs input` rows are brightest, `stopped` rows are nearly invisible.

### Project row

```
[▼] [󰉋] [project-name] [badge?] [session-count]
```

- **badge** — only shown when the project has ≥1 session in an alert state:
  - `⚠ N input` (yellow) when any session is `needs input`
  - `✽ N working` (blue) when working sessions exist and no input-needed
  - No badge when all sessions are idle/completed/stopped
- Project row left-border: yellow tint when needs-input badge shown, blue tint when working-only

### Top bar

```
◈ cleo  ·  N projects · N sessions  ·  [⚠ N needs input]  [✽ N working]       ♪ on · MEM
```

- `⚠ N needs input` badge: yellow background, black text, pulsing animation — only shown when N > 0
- `✽ N working` badge: dim blue border — only shown when N > 0
- Idle, completed, failed, stopped counts are not shown in the top bar (too noisy)

### Selected session — right pane header strip

```
[state-icon]  [agent-badge]  [session-name]  ·  [state-label]     [⚒ N tools] · [age] · [enter attach]
```

The attach CTA (`enter attach`) is shown for all states except `stopped`. Color follows state.

### Bottom bar

- `enter` attach — always present (greyed if stopped selected)
- `n` new session
- `K` kill
- `P` prune stopped — label clarifies it only removes stopped sessions
- `m` mute
- `?` help

---

## Agent Icon Config

Add an optional `icon` field to each `[agents.*]` block in `config.toml`:

```toml
[agents.claude]
command = "claude"
label   = "cl"
icon    = "◈"       # optional — nerd font character
color   = "#CC785C"
hooks   = "claude"

[agents.codex]
command = "codex"
label   = "cx"
icon    = "⬡"       # optional
color   = "#10A37F"
hooks   = "codex"

[agents.opencode]
command = "opencode"
label   = "oc"
# icon not set — falls back to label
color   = "#FF6B35"
hooks   = "none"
```

**Render logic:** if `icon` is non-empty, use it in the agent badge; otherwise use `label`. Badge width is fixed (accommodates 1–3 chars). No changes to hook system, session IDs, or any non-display code.

**Documentation:** Nerd font icons require a Nerd Font to be installed (`Hack Nerd Font`, `JetBrains Mono Nerd Font`, etc.). Document this as a soft dependency — cleo works without it, icons are purely cosmetic.

---

## State Machine Changes

### Reconciler additions

The reconciler (`internal/reconcile/reconcile.go`) already runs periodic tmux liveness checks. Three changes:

1. **Keep `idle → completed` auto-timeout (10 min default).** `idle` means "just finished a turn, expecting more work soon." `completed` means "has been idle long enough to be considered done — but tmux is still alive and you can attach and continue." The existing `[retention].idle_to_completed_timeout` (default 10 min) drives this transition and stays. `SessionEnd` also moves directly to `completed`.

   Session lifecycle:
   - `running` → Stop hook → `idle` (turn done, agent alive)
   - `idle` → new prompt → `running` (next turn)
   - `idle` → 10 min elapses → `completed` (tmux alive → green ✓, still attachable)
   - `idle` → SessionEnd fires → `completed` (same result, faster path)
   - `completed` → tmux dies → `stopped` (reconciler detects, grey ○, prune only)
   - `idle` → Ctrl+C (no SessionEnd) → tmux dies → `stopped` directly

2. **`completed`/`error` → `stopped` transition:** After a session reaches `completed` or `error`, the reconciler continues checking tmux liveness on each tick. When tmux disappears, apply `EvDead` (synthetic) to transition to `stopped` (internal state name `dead` stays unchanged — this is display-layer only).

3. **Sort order in state store:** No change to state.json format. Sorting is a TUI-only concern — applied at render time in `sidebar.go`.

### Display-only rename

The internal state machine enum values (`spawning`, `running`, `waiting_for_input`, `idle`, `completed`, `error`, `dead`) are **not renamed**. The mapping from internal state → display label happens only in the TUI layer (`sidebar.go`, `styles.go`). This avoids touching the hook handler, state store, reconciler, or any event logic.

---

## Out of Scope

- **Tmux preview pane redesign** — separate discussion
- **Inline reply / quick-approve from dashboard** — deferred (decided to handle via attach for now)
- **Cost / token count per session** — requires tracking new data from agent hooks; future feature
- **Clean exit vs crash detection** — both route to `stopped`; distinguishing is deferred
- **External notifications** (macOS Notification Center, webhooks) — future feature

---

## Testing

**Alert dedup:** Install hooks via `cleo init`, run `cleo init` again, verify no duplicate entries appear in `~/.claude/settings.json`. Trigger a `Notification` hook event, verify sound plays exactly once.

**Focus TTL:** Manually set a session as focused in `focus.json`, wait 5+ minutes without touching tmux, trigger a sound event, verify it plays (TTL expired).

**Hook-trace sound log:** Trigger events in each suppression scenario (focused, idle-nudge, disabled, enabled), read `hook-trace.log`, verify each decision is logged with reason.

**State transitions:** Spawn a session, let it reach `completed`, verify it shows green and `enter attach` is offered. Kill the tmux session manually, verify reconciler transitions it to `stopped` within one reconcile tick.

**Session sort order:** Spawn three sessions in a project — one idle, one needs-input, one working. Verify sidebar order is: needs-input → working → idle regardless of creation order.

**Agent icon fallback:** Set `icon = "◈"` for claude in config, verify badge shows `◈`. Remove `icon` field, verify badge falls back to `label = "cl"`.
