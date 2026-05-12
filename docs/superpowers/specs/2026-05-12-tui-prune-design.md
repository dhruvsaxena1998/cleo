# TUI Prune Feature Design

**Date:** 2026-05-12  
**Status:** Approved

## Summary

Add a `P` keybind to the TUI that prunes all finished sessions for the project currently in context (whether cursor is on the project row or one of its sessions). Shows a confirm popup before acting. Delegates all prune logic inline using the same state/events primitives the CLI uses — no subprocess, no new business logic.

## Scope

- `P` keybind triggers project-level prune (all finished sessions in the project at cursor)
- Confirm popup shows count before acting
- No-op if the project has zero finished sessions
- Footer hints updated to surface `P` when relevant

Out of scope: global (all-projects) prune, `--keep N` retention logic, dry-run mode.

## Files Changed

### `internal/tui/keymap.go`

Add `Prune key.Binding` to the `Keymap` struct and `DefaultKeymap()`:

```go
Prune: key.NewBinding(key.WithKeys("P"), key.WithHelp("P", "prune finished")),
```

### `internal/tui/popup_confirm.go`

Add a `title string` field to `ConfirmPopup` so the same popup renders "Confirm Kill" or "Confirm Prune". Update `NewConfirmPopup` signature:

```go
func NewConfirmPopup(title, actionLabel, prompt, target string, theme Theme) ConfirmPopup
```

- `title` — popup header text (e.g. `"Confirm Kill"`, `"Confirm Prune"`)
- `actionLabel` — the `y` key hint text in the footer (e.g. `"confirm kill"`, `"confirm prune"`)

All existing `NewConfirmPopup(prompt, target, theme)` call sites updated to pass `"Confirm Kill"` and `"confirm kill"` as the first two arguments. Prune passes `"Confirm Prune"` and `"confirm prune"`.

### `internal/tui/handle_key.go`

**Wire key in `handleKey`:**

```go
case key.Matches(msg, km.Prune):
    return m.confirmPrune()
```

**`confirmPrune()` method:**

```go
func (m Model) confirmPrune() (Model, tea.Cmd) {
    pid, ok := m.projectAtCursor()
    if !ok {
        return m, nil
    }
    var count int
    for _, s := range m.sessions {
        if s.ProjectID == pid && s.State.IsFinished() {
            count++
        }
    }
    if count == 0 {
        m.status = "no finished sessions to prune"
        return m, nil
    }
    m.status = ""
    prompt := fmt.Sprintf("prune %d finished session(s) in %q?", count, pid)
    m.popup = NewConfirmPopup("Confirm Prune", prompt, pid, m.theme)
    m.mode = ModePopup
    return m, m.popup.Init()
}
```

**`performPrune(projectID string)` method:**

```go
func (m Model) performPrune(projectID string) (Model, tea.Cmd) {
    for _, s := range m.sessions {
        if s.ProjectID != projectID || !s.State.IsFinished() {
            continue
        }
        _ = events.Archive(m.ctx.Paths.EventsLog(s.ID), m.ctx.Paths.ArchiveDir())
        _ = m.ctx.State.Delete(s.ID)
    }
    m.mode = ModeNormal
    m.popup = nil
    return m, loadStateCmd(m.ctx)
}
```

### `internal/tui/update.go`

Add new message type and dispatch:

```go
type PruneConfirmed struct{ ProjectID string }
```

In `Update`:

```go
case PruneConfirmed:
    return m.performPrune(msg.ProjectID)
```

`ConfirmYes` currently routes to `performKill`. Since we now have two confirm flows using the same popup, we need to distinguish them. Two options:

- **Option 1:** Replace `ConfirmYes{Target string}` with typed messages at dispatch time by threading a `kind` through `ConfirmPopup` — but that couples the popup to its outcome.
- **Option 2 (chosen):** Keep `ConfirmYes{Target}` and add a `kind` field to `ConfirmPopup` that is echoed back in a wrapper. Specifically: add `kind string` to `ConfirmPopup`; when `"y"` is pressed, emit `ConfirmYes{Target, Kind}`. In `update.go`, dispatch on `Kind`:

```go
case ConfirmYes:
    switch msg.Kind {
    case "prune":
        return m.performPrune(msg.Target)
    default:
        return m.performKill(msg.Target)
    }
```

This keeps the single confirm popup reusable without creating a parallel message hierarchy.

### `internal/tui/view.go` (footer)

Add `P` hint in the footer bar when the current project has finished sessions:

- **Project selected (no session cursor):** append `m.theme.KeyHint("P", "prune")` after `n` if the project has any finished sessions.
- **Session selected (finished):** append `m.theme.KeyHint("P", "prune project")` alongside the existing `K remove` hint.
- **Session selected (running):** omit `P` (no finished sessions are the target when a live session is selected — user can still press `P` and it'll work, but we don't clutter the footer).

## Message Flow

```
P pressed
  → confirmPrune()
    → count finished sessions for project
    → if 0: set status "no finished sessions to prune", return
    → open ConfirmPopup("Confirm Prune", prompt, projectID, kind="prune")
      → user presses y
        → ConfirmYes{Target: projectID, Kind: "prune"}
          → performPrune(projectID)
            → archive + delete each finished session
            → loadStateCmd
      → user presses esc/n
        → ConfirmNo → close popup
```

## Behaviour Notes

- `P` while cursor is on a session row prunes the **parent project** (resolved via `projectAtCursor()`).
- `P` with no finished sessions sets a status message and does nothing else.
- After prune completes, state reloads and cursor clamps to valid position via existing `clampCursor()`.
- Events are archived before deletion (matches CLI `prune` behaviour).
- The retention banner hint (`run: cleo prune <project>`) remains — it's a different path (threshold-based) and still valid for users who prefer the CLI.

## Non-Goals

- No `--keep N` retention: TUI prune removes **all** finished sessions for the project.
- No dry-run mode in TUI.
- No global (all-projects) prune in this iteration.
