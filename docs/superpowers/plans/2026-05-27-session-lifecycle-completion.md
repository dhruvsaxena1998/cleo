# Session Lifecycle Completion Implementation Plan

> **For agentic workers:** use red-green-refactor. Move one lifecycle behaviour at a time, update both callers, then stop and run tests. Do not mix Worktree work into this diff.

**GitHub issue:** https://github.com/dhruvsaxena1998/cleo/issues/52

**Goal:** Finish deepening Cleo's Session lifecycle module so CLI and TUI share Session lifecycle rules for attach/revive, kill, prune, rename, and Project Session removal.

**Architecture:** `internal/sessionlifecycle` becomes the deep module for Session lifecycle behaviour. CLI and TUI retain user interaction and terminal process execution. Session lifecycle owns state, tmux side effects, event-log archive/removal, and typed outcomes.

**Tech Stack:** Go, Cobra CLI, Bubble Tea TUI, file-backed Project/Session stores, tmux adapter.

---

## Background

The creation tracer-bullet proved the Session lifecycle seam. `cleo run` and TUI spawn now call `internal/sessionlifecycle.Create` instead of duplicating Project resolution, Session naming, state-first creation, tmux launch, rollback, focus hooks, and detach-key binding.

The remaining lifecycle behaviours are still shallow in callers:

- `internal/cli/kill.go` and `internal/tui/handle_key.go` both know tmux kill + state deletion rules.
- `internal/cli/prune.go` and `internal/tui/handle_key.go` both know finished Session pruning and event-log archive rules.
- `internal/cli/rename.go` and `internal/tui/handle_key.go` both slugify names and write state directly.
- `internal/cli/rm.go` and `internal/tui/handle_key.go` both remove Project Session records directly.
- `internal/tui/handle_key.go` owns attach/revive rules that `internal/cli/attach.go` does not share.

The deletion test points to the same answer for each: removing the caller code does not remove complexity; it should concentrate behind the Session lifecycle interface.

---

## Files

- Modify: `internal/sessionlifecycle/create.go` or split into new files under `internal/sessionlifecycle/`
- Add: `internal/sessionlifecycle/kill_test.go`, `prune_test.go`, `rename_test.go`, `attach_test.go`, `remove_project_test.go` as needed
- Modify: `internal/cli/attach.go`
- Modify: `internal/cli/kill.go`
- Modify: `internal/cli/prune.go`
- Modify: `internal/cli/rm.go`
- Modify: `internal/cli/rename.go`
- Modify: `internal/tui/handle_key.go`
- Modify caller tests under `internal/cli/` and `internal/tui/` as needed
- Possibly modify: `internal/cli/tmux_iface.go` if the tmux seam needs a narrower method set

---

## Design constraints

- Preserve existing user-facing behaviour unless explicitly called out in the PRD.
- Keep prompts, popups, status text, and rendering in CLI/TUI.
- Keep terminal attach process execution in CLI/TUI.
- Keep concrete Project and Session stores unless tests prove a seam is needed.
- Keep tmux as the real external adapter seam.
- Do not introduce Worktree code.
- Prefer typed lifecycle outcomes over caller-specific strings.
- Add lifecycle tests before moving caller code.
- Run `go test ./...` after each migration slice.

---

## Proposed module shape

Exact names can change during implementation, but preserve the leverage:

- Extend `sessionlifecycle.Options` with any needed concrete dependencies:
  - `Paths paths.Paths` or explicit event-log/archive paths helper
  - `Focus` store or focus setter if attach focus moves in
- Keep existing `Lifecycle` constructor.
- Add typed outcome structs:
  - `KillResult { SessionID string; Warning error }`
  - `PruneResult { SessionIDs []string; Warnings []error }`
  - `RenameResult { Session state.Session; OldName string; NewName string }`
  - `PrepareAttachResult { Session state.Session; Action AttachAction; Message string }`
  - `RemoveProjectSessionsResult { ActiveCount int; FinishedCount int; RemovedSessionIDs []string; Warnings []error }`
- Add sentinel errors or typed reason values for:
  - Session not found
  - Active Sessions block Project removal
  - Attach blocked because Session is dead/errored
  - Attach target missing; Session marked dead
- Avoid forcing CLI/TUI to inspect state transitions directly.

---

## Task 1: Prepare lifecycle test harness

**Files:**
- Add shared lifecycle package test helpers if useful
- Modify `internal/sessionlifecycle/create_test.go` only if helpers reduce duplication

- [ ] Add a fake tmux adapter that supports `NewSession`, `HasSession`, `Kill`, `InstallFocusHooks`, and optional failure injection.
- [ ] Add helper to create temp `paths.Paths`, Project store, Session store, and Lifecycle.
- [ ] Add helper to write Session event logs and assert active/archive paths.
- [ ] Keep existing creation tests green.

Run:

```bash
go test ./internal/sessionlifecycle
```

---

## Task 2: Move kill Session into Session lifecycle

**Files:**
- Add/modify lifecycle kill implementation
- Add `internal/sessionlifecycle/kill_test.go`
- Modify `internal/cli/kill.go`
- Modify `internal/tui/handle_key.go`
- Modify kill-related tests

### Red tests

- [ ] Kill existing running Session calls tmux kill and deletes Session record.
- [ ] Kill existing finished Session deletes Session record; tmux kill may be skipped or harmless.
- [ ] Kill unknown Session returns typed not-found outcome.
- [ ] tmux kill failure returns warning outcome but still deletes Session record.

### Implementation

- [ ] Add `Lifecycle.Kill(sessionID string)`.
- [ ] Move state lookup, tmux kill, and state deletion into lifecycle.
- [ ] Preserve CLI confirmation outside lifecycle.
- [ ] Preserve CLI warning output for tmux failure.
- [ ] Preserve TUI confirmation popup outside lifecycle.
- [ ] TUI `performKill` calls lifecycle and reloads state.

Run:

```bash
go test ./internal/sessionlifecycle ./internal/cli ./internal/tui
go test ./...
```

---

## Task 3: Move prune into Session lifecycle

**Files:**
- Add/modify lifecycle prune implementation
- Add `internal/sessionlifecycle/prune_test.go`
- Modify `internal/cli/prune.go`
- Modify `internal/tui/handle_key.go`
- Modify prune-related tests

### Red tests

- [ ] Prune selects only finished Sessions.
- [ ] Prune with Project filter only prunes that Project.
- [ ] Prune with keep count keeps N most recent finished Sessions per Project.
- [ ] Prune archives active event logs before deleting Session records.
- [ ] Archive failure returns warning but does not block remaining deletes, unless existing behaviour requires hard failure.

### Implementation

- [ ] Add `Lifecycle.Prune(input)` with mode fields for `ProjectID`, `Keep`, and `AllProjects`.
- [ ] Move candidate selection and event archive/delete into lifecycle.
- [ ] CLI keeps dry-run rendering and confirmation.
- [ ] CLI can call lifecycle candidate selection for dry-run, or lifecycle can return preview candidates without side effects.
- [ ] TUI selected-project prune calls lifecycle with ProjectID and keep=0/all-finished mode.
- [ ] Remove direct `events.Archive` and `State.Delete` calls from TUI prune path.

Run:

```bash
go test ./internal/sessionlifecycle ./internal/cli ./internal/tui
go test ./...
```

---

## Task 4: Move rename into Session lifecycle

**Files:**
- Add/modify lifecycle rename implementation
- Add `internal/sessionlifecycle/rename_test.go`
- Modify `internal/cli/rename.go`
- Modify `internal/tui/handle_key.go`
- Modify rename-related tests

### Red tests

- [ ] Rename slugifies the requested display name.
- [ ] Rename returns old and new names.
- [ ] Rename unknown Session returns typed not-found outcome.
- [ ] Rename updates only display name; Session ID is unchanged.

### Implementation

- [ ] Add `Lifecycle.Rename(sessionID, newName string)`.
- [ ] Move slugification and state write into lifecycle.
- [ ] CLI renders `renamed <id>: <old> → <new>` from lifecycle result.
- [ ] TUI popup still blocks finished Sessions before submit, or lifecycle returns a typed blocked result if that rule is moved inside.
- [ ] Remove direct `ids.Slugify` and `State.Put` from caller rename paths.

Run:

```bash
go test ./internal/sessionlifecycle ./internal/cli ./internal/tui
go test ./...
```

---

## Task 5: Move attach preparation and revive into Session lifecycle

**Files:**
- Add/modify lifecycle attach implementation
- Add `internal/sessionlifecycle/attach_test.go`
- Modify `internal/cli/attach.go`
- Modify `internal/tui/handle_key.go`
- Modify attach-related tests

### Red tests

- [ ] Prepare attach blocks dead Session.
- [ ] Prepare attach blocks errored Session.
- [ ] Prepare attach on missing tmux marks Session dead and returns marked-dead outcome.
- [ ] Prepare attach on completed-but-live Session applies user-resume and returns attachable outcome.
- [ ] Prepare attach on running/live Session returns attachable outcome.
- [ ] Prepare attach installs focus hooks when supported.
- [ ] Focus begin/end set and clear focus state if focus dependency is provided.

### Implementation

- [ ] Add `Lifecycle.PrepareAttach(sessionID string)`.
- [ ] Add `Lifecycle.SetFocused(sessionID string, focused bool)` or equivalent focus helper.
- [ ] Move TUI liveness check and completed revive into lifecycle.
- [ ] CLI attach calls `PrepareAttach` before starting tmux attach.
- [ ] CLI attach keeps running `exec.Command("tmux", "attach", "-t", id)`.
- [ ] TUI attach keeps running `tea.ExecProcess(exec.Command("tmux", "attach", "-t", id), ...)`.
- [ ] Both callers set focus through lifecycle before/after attach.
- [ ] TUI status messages are mapped from typed attach outcomes.

Run:

```bash
go test ./internal/sessionlifecycle ./internal/cli ./internal/tui
go test ./...
```

---

## Task 6: Move Project Session removal into Session lifecycle

**Files:**
- Add/modify lifecycle Project removal implementation
- Add `internal/sessionlifecycle/remove_project_test.go`
- Modify `internal/cli/rm.go`
- Modify `internal/tui/handle_key.go`
- Modify remove-related tests

### Red tests

- [ ] Remove Project Sessions classifies active and finished Sessions.
- [ ] Remove without force returns blocked outcome when active Sessions exist and deletes nothing.
- [ ] Remove with force kills active Sessions best-effort.
- [ ] Remove deletes Session records for the Project.
- [ ] Remove archives/removes active event logs for those Sessions.
- [ ] tmux kill failures are returned as warnings but do not block forced removal.
- [ ] Project removal itself remains caller-owned or is done by lifecycle only if the module receives the Project store.

### Implementation

- [ ] Add `Lifecycle.ProjectSessionSummary(projectID string)` if callers need counts before prompting.
- [ ] Add `Lifecycle.RemoveProjectSessions(input)` with `ProjectID` and `Force`.
- [ ] Move active/finished classification into lifecycle.
- [ ] Move best-effort active tmux kill into lifecycle.
- [ ] Move Session record deletion into lifecycle.
- [ ] Move event-log archive/removal into lifecycle.
- [ ] Decide whether lifecycle also calls `Projects.Remove(projectID)` or returns success so caller removes Project. Prefer lifecycle owning it if it receives Project store and the behaviour is named `RemoveProject`.
- [ ] Preserve CLI `--force` and `--yes` prompt behaviour.
- [ ] Preserve TUI remove confirmation behaviour.

Run:

```bash
go test ./internal/sessionlifecycle ./internal/cli ./internal/tui
go test ./...
```

---

## Task 7: Clean up shallow caller code

**Files:**
- `internal/cli/*.go`
- `internal/tui/handle_key.go`
- lifecycle package files

- [ ] Remove direct lifecycle state mutations from CLI/TUI where lifecycle methods now exist.
- [ ] Remove direct `events.Archive` calls from TUI/CLI prune/remove paths where lifecycle owns them.
- [ ] Remove direct `ids.Slugify` from caller rename paths.
- [ ] Remove direct attach/revive state logic from TUI.
- [ ] Ensure lifecycle package does not import CLI or TUI.
- [ ] Ensure there is no import cycle.
- [ ] Keep caller tests focused on rendering/output, not lifecycle internals.

Run:

```bash
go test ./...
```

---

## Task 8: Documentation and regression pass

- [ ] Update `CONTEXT.md` only if Session lifecycle wording needs sharpening.
- [ ] Verify no Worktree implementation was introduced.
- [ ] Run `gofmt` on modified Go files.
- [ ] Run `go test ./...`.
- [ ] Manually smoke-test if practical:
  - `cleo run <agent> --no-attach --yes`
  - `cleo attach <session-id>`
  - `cleo kill <session-id> --yes`
  - `cleo prune --dry-run`
  - TUI kill/prune/rename/remove flows

---

## Acceptance criteria

- CLI and TUI use Session lifecycle for kill.
- CLI and TUI use Session lifecycle for prune.
- CLI and TUI use Session lifecycle for rename.
- CLI and TUI use Session lifecycle for attach preparation/revival/focus.
- CLI and TUI use Session lifecycle for Project Session removal.
- Project removal removes Session records and active event logs for the Project's Sessions.
- Tmux kill failures become warnings where cleanup should still proceed.
- Lifecycle tests cover state, tmux, and event-log outcomes.
- Existing caller behaviour is preserved except for documented Project event-log cleanup.
- `go test ./...` passes.

---

## Suggested commit order

1. `test(sessionlifecycle): add harness for remaining lifecycle behaviours`
2. `feat(sessionlifecycle): move kill behaviour behind lifecycle module`
3. `feat(sessionlifecycle): move prune behaviour behind lifecycle module`
4. `feat(sessionlifecycle): move rename behaviour behind lifecycle module`
5. `feat(sessionlifecycle): move attach preparation behind lifecycle module`
6. `feat(sessionlifecycle): move project session removal behind lifecycle module`
7. `refactor(cli,tui): remove duplicated lifecycle rules`
8. `docs: add session lifecycle completion plan`
