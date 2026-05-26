# TUI Add Project — Design Spec

**Date:** 2026-05-25  
**Status:** Approved

## Problem Statement

Users can register projects from the CLI via `cleo add <path>`, but there is no way to add a project from within the TUI. The only way to spawn an agent in a new project from the TUI is to register it from the CLI first, then return to the TUI. This breaks the TUI-first workflow and forces context-switching to the terminal.

## Solution

Extend the existing spawn popup (`n` key) to include an editable **path** field. The spawn popup becomes a unified surface that can either spawn into an existing project (prefilled path) or register a new project and spawn into it (edited path). No new keybinding is needed — `n` gains project registration as an implicit capability.

## User Stories

1. As a Cleo TUI user, I want to spawn an agent in a new project without leaving the TUI, so that I don't have to context-switch to the terminal.
2. As a Cleo TUI user, I want the path field pre-filled with the current project when I press `n`, so that spawning into the current project is zero extra keystrokes.
3. As a Cleo TUI user, I want to edit the path field to point to a different existing project, so that I can switch projects without navigating the sidebar.
4. As a Cleo TUI user, I want to type a path to a directory that isn't a registered project and have Cleo register it automatically, so that adding and using a project is a single action.
5. As a Cleo TUI user, I want to see an inline error in the popup if I enter an invalid path (non-existent, not a directory), so that I can correct it without losing my other field values.
6. As a Cleo TUI user, I want the agent selector to pre-select the project's `default_agent` (or the global default), so that I can hit Enter immediately for the common case.
7. As a Cleo TUI user, I want the initial focus to be on the label field when a project exists, so that I can Tab past the already-correct path quickly.
8. As a Cleo TUI user, I want Tab to cycle through all three fields (path → label → agents → path), so that navigation is predictable and linear.
9. As a Cleo TUI user, I want to see a descriptive preview ("will register project, then create session") when the path points to a new project, so that I know what will happen on submit.
10. As a Cleo TUI user, I want to see a concrete session ID preview when the path matches an existing project, so that I know which project I'm targeting.
11. As a Cleo TUI user, I want an empty tree (no projects) to still open a spawn popup with the CWD as the default path, so that I can add my first project from the TUI.
12. As a Cleo TUI user, I want the popup to validate that the path is a real directory before submitting, so that I don't create phantom project entries.
13. As a Cleo TUI user, I want the popup to check if the path is already registered and simply target that project if it is, so that duplicate projects are never created.

## Implementation Decisions

### Spawn popup layout: three-zone Tab cycle

The popup now has three sections navigated by `Tab`:

```
┌──────────────────────────────────────────────────────────────────┐
│ New Session                                spawn tmux-backed agent │
├──────────────────────────────────────────────────────────────────┤
│                                                                  │
│  1. path                                                          │
│     › /Users/dhruvsaxena/Dev/myapp                               │
│                                                                  │
├──────────────────────────────────────────────────────────────────┤
│                                                                  │
│  2. label  (optional)                                            │
│     › [________________________]                                 │
│                                                                  │
├──────────────────────────────────────────────────────────────────┤
│                                                                  │
│  3. ai-agent                                                      │
│     ● claude                                                      │
│     ○ codex                                                      │
│     ○ opencode                                                   │
│     ○ pi                                                          │
│                                                                  │
├──────────────────────────────────────────────────────────────────┤
│                                                                  │
│  will register project, then create session                      │
│  $ tmux new-session -d -s cleo-<id>-<agent>-<name> <command>     │
│                                                                  │
├──────────────────────────────────────────────────────────────────┤
│  tab next field  ·  j/k move agents  ·  enter spawn  ·  esc cancel│
└──────────────────────────────────────────────────────────────────┘
```

- **Order:** path → label → agents. Mandatory before optional. The most impactful decision (which project) comes first.
- **Tab cycles linearly:** path → label → agents → path. Shift+Tab cycles backwards.
- **Popup width:** increased from 52 to 64 chars to accommodate file paths.

### Path input field

- Uses `bubbles/textinput.Model` with `Width` set to ~60 (fits within 64-wide popup with borders and padding).
- `textinput` handles horizontal scrolling when the cursor moves past the visible width.
- Placeholder text: `enter project path` when empty.

### Default values and initial focus

| Scenario | Path default | Agent default | Initial focus |
|---|---|---|---|
| `n` on project row | Current project's `Path` | `Project.DefaultAgent` → `Config.DefaultAgent` fallback | Label field |
| `n` on session row (sibling) | Session's project `Path` | `Project.DefaultAgent` → `Config.DefaultAgent` fallback | Label field |
| `n` on empty tree (no projects) | Current working directory | `Config.DefaultAgent` | Path field (mandatory, empty) |

- When path is prefilled and valid, focus starts on label — the user's common intent is to customize the session name or pick an agent, not change the project.
- When path is empty (no project context), focus starts on path — the mandatory field must be filled first.

### SpawnSubmitted message change

The `SpawnSubmitted` struct is extended with `Path` and retains `ProjectID`:

```go
type SpawnSubmitted struct {
    ProjectID string  // resolved from popup; empty string if path is new
    Path      string  // raw path from input
    Agent     string
    Name      string
}
```

- `SpawnPopup` constructor receives the project list (`[]projects.Project`) so it can resolve: if the path matches an existing project's `Path`, `ProjectID` is set to that project's ID. Otherwise `ProjectID` is `""`.
- In `performSpawn`, the handler checks: if `ProjectID` is empty, call `m.ctx.Projects.Add(msg.Path)` to register the project, then use the returned `Project.ID`.

### Validation on submit

The popup validates on Enter press:

1. **Empty path** → inline error: `path is required`
2. **Path does not exist on disk** → inline error: `directory not found`
3. **Path exists but is not a directory** → inline error: `not a directory`
4. **Path is an existing directory, already registered** → valid: spawn into that project (resolved by matching path to existing project)
5. **Path is an existing directory, not registered** → valid: register project, then spawn
6. **Path is an existing empty directory** → valid: same as 5

Errors appear as red text on the line directly below the path input. The popup stays open for correction.

### Session ID preview

- **Path matches existing project:** show concrete session ID and command (same as current behavior), e.g. `cleo-myapp-claude-1`.
- **Path is new/unregistered:** show `will register project, then create session` in place of the session ID line. The command shows the agent command only (project ID is unknown until registration).

### Error display in popup

A new error state field on `SpawnPopup`:

```go
type SpawnPopup struct {
    // ... existing fields ...
    pathError string  // non-empty when path validation fails
}
```

When `pathError` is non-empty, an additional line appears below the path input with the error in red/styled text. The error is cleared whenever the path input changes.

### Auto-agent selection

- When `ProjectID` resolves to an existing project with a `DefaultAgent` set, pre-select that agent in the list.
- When `ProjectID` is empty (new project) or the project has no `DefaultAgent`, pre-select `Config.DefaultAgent`.
- When the user edits the path and the matched project changes, the agent cursor should remain on the currently selected agent (no jarring re-selection).

### Open when no project exists

Currently `openSpawnPopup()` returns early (no-op) when `projectAtCursor()` fails. This guard is removed. Instead:

- If a project exists at cursor → prefilled path, focus on label.
- If no project at cursor (empty tree) → path defaults to CWD (resolved via `os.Getwd()`), focus on path.

This ensures `n` always opens the popup.

### Footer hints

Footer hints outside the popup remain unchanged (`n new session` / `n new sibling`). Inside the popup, the footer updates to:

```
tab next field  ·  j/k move agents  ·  enter spawn  ·  esc cancel
```

### Actions panel (sidebar)

The "Actions" panel in the sidebar currently shows contextual actions. When no session is selected, it shows:

```
  › spawn new agent        n
    filter sessions        /
    expand / collapse      space
    quit                   q
```

No changes needed — `n` is already surfaced. The path field's dual capability is discoverable inside the popup itself.

## Modules Modified

### `internal/tui/popup_spawn.go`

- `SpawnPopup` struct gains: `pathInput textinput.Model`, `projects []projects.Project`, `pathError string`, `cwd string`, `focusIndex int` (replaces `focusName bool` — now 0=path, 1=label, 2=agent).
- `NewSpawnPopup` signature changes to accept the project list, CWD, resolved default project ID, and resolved default agent key.
- `View()` method rewritten to render three sections with dividers, inline error, and preview.
- `Update()` method handles Tab cycling through three focus zones, validates path on Enter, and resolves ProjectID from path against project list.

### `internal/tui/update.go`

- `SpawnSubmitted` struct gains `Path string` and `ProjectID string` fields; `ProjectID` may be empty for new projects.
- `performSpawn` handler: if `msg.ProjectID` is empty, call `m.ctx.Projects.Add(msg.Path)` to register the project, then proceed with spawning.

### `internal/tui/handle_key.go`

- `openSpawnPopup()`: remove the early-return guard when no project is at cursor. Instead, default to CWD path and empty `ProjectID`.
- Pass project list, CWD, and default agent info to `NewSpawnPopup`.

### `internal/tui/view.go`

- No changes to footer hints outside the popup.

### `internal/projects/store.go`

- May need a `FindByPath(path string) (Project, error)` convenience method, or the TUI can do the lookup inline. The lookup is: iterate `m.projects` (already on the model from `stateLoadedMsg`) and match `Path` against the input. No store change needed.

### `internal/cli/add.go`

- No changes. CLI `cleo add` remains as-is.

## Testing Decisions

### What makes a good test

- Test external behaviour (what the popup renders, what messages it emits), not internal implementation details.
- Test validation logic: empty path, non-existent path, non-directory path, duplicate path, valid new path.
- Test Tab cycling between three focus zones.
- Test that default values are correctly computed for each scenario (project row, session row, empty tree).

### Modules to test

1. **`popup_spawn.go`** — Unit tests for:
   - Validation logic on submit (all six cases from the validation table).
   - Tab cycling (focusIndex 0→1→2→0, and Shift+Tab 0→2→1→0).
   - Default agent selection from project default → config default fallback.
   - Path-to-project resolution (existing project matches, new path doesn't).

2. **`handle_key.go`** / `update.go` — Integration-style tests for:
   - `openSpawnPopup()` with no project at cursor → popup opens with CWD, focus on path.
   - `openSpawnPopup()` with project at cursor → popup opens with project path, focus on label.
   - `performSpawn()` with new path → project registered, then session spawned.
   - `performSpawn()` with existing project path → no new project registered, session spawned.

### Prior art

- `internal/cli/add_test.go` — existing tests for the CLI `add` command.
- `internal/tui/tui_test.go` — existing snapshot tests for TUI rendering.
- `internal/projects/projects_test.go` — existing tests for the project store (Add, Remove, List).

## Out of Scope

- **Path tab-completion / intellisense** — deferred to a future iteration. The initial implementation uses a plain text input. Shell-style directory tab completion can be layered on later.
- **File picker / directory browser UI** — not in scope.
- **Inline config editor for project settings** (e.g. `default_agent`) — not in scope.
- **Removing projects from TUI** — already implemented via `D` key.
- **Global "add project" keybinding separate from `n`** — the unified spawn popup covers this case.
- **Auto-detection of projects** (scanning for common project markers like `.git`) — not in scope.

## Further Notes

- The spawn popup's `ProjectID` field uses the existing project list to resolve at popup time. If two projects share the same path (which Store.Add prevents), the lookup is deterministic (first match).
- After registering a new project, `loadStateCmd` will refresh the project list and the sidebar will show the new project immediately.
- The `projects.Store.Add()` method already handles deduplication (returns error if path already registered). The popup resolves existing paths before calling `Add`, so the error path should never be hit — but if it is, the inline error display will surface it.
- CWD resolution for the empty-tree case uses `os.Getwd()`. If this fails (unlikely), the path input starts empty and the user must type a path.