# Design: Fix `up/k` cursor navigation jump (issue #24)

**Date:** 2026-05-12
**Status:** Approved
**Labels:** bug, P1: High

## Problem

`cursorUp()` in `internal/tui/handle_key.go` does not mirror the behaviour of `cursorDown()`. When the cursor is on a project header row and the user presses `up/k`, it jumps directly to the previous project's header row instead of landing on the previous project's last session (when that project is expanded). `down/j` correctly steps through sessions before crossing a project boundary.

## Root cause

`cursorUp()` decrements `projectIdx` but never checks whether the new project is expanded or sets `agentIdx` to the last session index. `cursorDown()` does check expansion before crossing the project boundary, making the two directions asymmetric.

## Fix

**File:** `internal/tui/handle_key.go` — `cursorUp()` function only.

After decrementing `projectIdx`, look up the previous project's ID from `visibleProjectIDs()`, check `m.expanded`, and if the project is expanded with at least one session, set `agentIdx = len(ss) - 1`.

```go
if m.cursor.projectIdx > 0 {
    m.cursor.projectIdx--
    prevPID := m.visibleProjectIDs()[m.cursor.projectIdx]
    if m.expanded[prevPID] {
        if ss := m.sessionsFor(prevPID); len(ss) > 0 {
            m.cursor.agentIdx = len(ss) - 1
        }
    }
}
```

## Edge cases

| Scenario | Behaviour |
|---|---|
| Previous project is collapsed | `agentIdx` stays `-1`; lands on project header |
| Previous project expanded but all sessions filtered out | `len(ss) == 0`; agentIdx stays `-1`; lands on project header |
| Cursor at first project | `projectIdx == 0`; no movement |
| Filter active | `sessionsFor` already respects the filter; no special handling needed |

## Test

**File:** `internal/tui/tui_test.go` — add `TestCursorUpDownNavigation`.

Direct unit test (no teatest harness). Setup: 2 projects (`p1`, `p2`), each with 2 sessions, both expanded, cursor starting at `p2` header (projectIdx=1, agentIdx=-1).

Assert the following sequence:

| Press | projectIdx | agentIdx | Description |
|---|---|---|---|
| up | 0 | 1 | p1 last session |
| up | 0 | 0 | p1 first session |
| up | 0 | -1 | p1 project header |
| up | 0 | -1 | no movement (already at top) |

Then verify symmetry by navigating down from the top and confirming the path reverses correctly.

## Scope

- `internal/tui/handle_key.go` — one function, ~4 lines added
- `internal/tui/tui_test.go` — one new test function

No data model changes. No new types.
