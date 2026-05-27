# Session Lifecycle Completion PRD

**Date:** 2026-05-27  
**Status:** Ready for agent  
**GitHub issue:** https://github.com/dhruvsaxena1998/cleo/issues/52  
**Scope:** Complete the deep Session lifecycle module after the creation tracer-bullet

## Problem Statement

Cleo now routes Session creation through `internal/sessionlifecycle`, but the rest of the Session lifecycle still leaks across CLI and TUI callers.

Killing, pruning, renaming, attaching/reviving, and removing a Project's Sessions are implemented directly in caller modules. Each caller must know which Session states are terminal, when tmux should be killed, when a missing tmux Session means the Session should be marked dead, how completed Sessions can revive, how event logs are archived or removed, and how Session names are slugified.

That keeps the Session lifecycle shallow outside creation: the interface each caller must learn is nearly as complex as the implementation. The deletion test says these caller implementations are not earning their keep. Deleting one copy does not remove complexity; it reappears in the other caller or in tests. The Session lifecycle module should concentrate these rules so callers keep prompts, popups, output, and terminal process execution while the lifecycle implementation owns durable Session behaviour.

## Solution

Deepen `internal/sessionlifecycle` into the single module for Session lifecycle behaviours:

- create Session (already done)
- prepare attach and revive stale completed Sessions
- mark missing Sessions dead
- kill Session
- prune finished Sessions
- rename Session
- remove a Project's Sessions and remove their active event logs

CLI and TUI remain the user-interaction modules. They ask for consent, render status, and run terminal attach processes. Session lifecycle owns state changes, tmux side effects, event-log archive/removal, and typed outcomes.

The work should proceed in small vertical slices. Each slice moves one behaviour through the Session lifecycle interface, updates both CLI and TUI callers where applicable, and adds lifecycle tests before removing caller duplication.

## User Stories

1. As a Cleo user, I want killing a Session from CLI or TUI to behave the same, so that there is no caller-specific drift.
2. As a Cleo user, I want killing a Session to remove its Session record even if tmux kill fails, so that stale records can be cleaned up.
3. As a Cleo user, I want a warning when tmux kill fails, so that failures remain visible without blocking cleanup.
4. As a Cleo user, I want pruning finished Sessions from CLI or TUI to archive event logs and remove Session records consistently.
5. As a Cleo user, I want pruning to keep the configured number of recent finished Sessions per Project when using CLI defaults.
6. As a Cleo user, I want TUI project pruning to keep removing all finished Sessions in the selected Project after confirmation.
7. As a Cleo user, I want renaming a Session from CLI or TUI to slugify the new name consistently.
8. As a Cleo user, I want finished Sessions to remain unrenamable in the TUI, preserving the current interaction rule.
9. As a Cleo user, I want attaching to a dead or errored Session to stay blocked.
10. As a Cleo user, I want attaching to a Session whose tmux Session vanished to mark it dead.
11. As a Cleo user, I want attaching to a completed Session whose tmux Session is still alive to revive it.
12. As a Cleo user, I want focus tracking to be set while I am attached and cleared when I detach.
13. As a Cleo user, I want removing a Project to remove Session records for that Project.
14. As a Cleo user, I want removing a Project to remove active event logs for that Project's Sessions, matching the documented Session lifecycle rule.
15. As a Cleo user, I want Project removal without force to keep blocking when active Sessions exist.
16. As a Cleo user, I want forced Project removal to kill active Sessions best-effort and still unregister the Project.
17. As a Cleo maintainer, I want one Session lifecycle module to own tmux, state, and event-log side effects, so that caller code does not duplicate rules.
18. As a Cleo maintainer, I want typed lifecycle outcomes, so that CLI and TUI can render differently without duplicating lifecycle decisions.
19. As a Cleo maintainer, I want lifecycle tests to cover the interface, so that the test surface matches the module seam.
20. As a Cleo maintainer, I want no Worktree behaviour in this change, so that the Worktree concept does not distort the Session lifecycle seam.

## Implementation Decisions

- Keep `internal/sessionlifecycle` as the deep module name because `CONTEXT.md` already defines Session lifecycle.
- Extend the existing `Lifecycle` type rather than creating a second lifecycle module.
- Callers keep consent, prompts, popups, status text, terminal process execution, and rendering.
- Session lifecycle owns Session state mutations, tmux liveness checks, tmux kill, focus-hook installation when attaching, focus state set/clear helpers, event-log archive/removal, and Project Session cleanup.
- Attach should be split into lifecycle preparation and caller process execution:
  - lifecycle validates the Session and returns an attach target or a typed blocked outcome.
  - caller runs `tmux attach`, `tmux attach-session`, or `tmux switch-client` as today.
  - lifecycle provides begin/end focus operations or a small focus outcome that callers invoke around the attach process.
- TUI attach keeps its current user-facing rules: dead/errored are blocked; missing tmux marks dead; completed-but-live revives.
- CLI attach may gain the same validation/revival rules for consistency, but must still surface tmux attach errors from the process execution.
- Kill is best-effort against tmux: tmux failure becomes a warning outcome, while Session state deletion still happens if the Session exists.
- Prune archives event logs before deleting Session records, preserving current prune behaviour.
- Project removal removes Session records and removes active event logs for those Sessions. Archiving the active event log before deletion is acceptable because it removes the active log path while preserving diagnostics.
- Rename updates the Session display name only; it does not rename the tmux Session ID.
- Rename uses the same slugging rule in CLI and TUI.
- Store modules remain concrete file-backed modules; do not introduce store abstractions unless tests force them.
- The tmux seam remains the real external adapter seam.
- Event-log archive/removal may be a small internal seam for tests if direct filesystem assertions become noisy.
- Existing config schema and command flags do not change.
- Existing Worktree definition remains out of scope.

## Lifecycle Behaviours

### Kill Session

Input: Session ID.  
Implementation: get Session, best-effort tmux kill, delete Session record.  
Outcome: deleted Session ID plus optional warning if tmux kill failed.

### Prune Sessions

Input: optional Project filter, keep count, all-projects flag or explicit selected Project mode.  
Implementation: select finished Sessions, archive event logs, delete records.  
Outcome: pruned Session IDs and archive warnings if any.

### Rename Session

Input: Session ID and new display name.  
Implementation: load Session, slugify new name, update state.  
Outcome: Session ID, old name, new name.

### Prepare Attach

Input: Session ID.  
Implementation: load Session, block hard terminal states, check tmux liveness, mark missing as dead, revive completed-but-live, install focus hooks.  
Outcome: attach target, blocked reason, marked-dead result, or revived result.

### Focus Around Attach

Input: Session ID and focus boolean.  
Implementation: set focus state.  
Outcome: best-effort warning only.

### Remove Project Sessions

Input: Project ID and force flag.  
Implementation: classify active/finished Sessions, block active Sessions unless force, best-effort kill active Sessions when forced, archive/remove active event logs, delete Session records.  
Outcome: counts, removed Session IDs, active-blocked result, warnings.

## Testing Decisions

- Add focused tests in `internal/sessionlifecycle` for each lifecycle behaviour.
- Keep CLI/TUI tests as caller contract tests: prompts/status/output remain stable, but lifecycle rules are tested once in the lifecycle package.
- Test kill deletes Session state even when tmux kill fails and returns a warning outcome.
- Test kill of unknown Session returns a typed not-found outcome.
- Test prune selects only finished Sessions and respects keep count per Project.
- Test prune archives event logs and deletes Session records.
- Test rename slugifies the new name and returns old/new names.
- Test prepare attach blocks dead and errored Sessions.
- Test prepare attach marks missing tmux Sessions dead.
- Test prepare attach revives completed-but-live Sessions.
- Test attach focus begin/end writes focus state where practical.
- Test remove Project Sessions blocks active Sessions without force.
- Test remove Project Sessions kills active Sessions with force and returns warnings on tmux kill failure.
- Test remove Project Sessions removes Session records and active event logs.
- Run `go test ./...` after each vertical slice.

## Out of Scope

- Worktree creation, cleanup, or branch management.
- Changing Session ID format.
- Renaming tmux Sessions when display names change.
- Changing command flags or TUI keybindings.
- Changing the Session state machine unless a lifecycle test exposes a real bug.
- Reworking Project registration.
- Reworking hook protocol normalization.
- Replacing file-backed Project or Session stores with abstract store seams.
- Changing event retention policy beyond Project removal matching the documented Session lifecycle rule.

## Acceptance Criteria

- `internal/sessionlifecycle` owns create, attach preparation/revival, kill, prune, rename, and Project Session removal.
- CLI and TUI no longer duplicate tmux/state/event-log lifecycle rules.
- CLI and TUI retain prompts, popups, status text, rendering, and terminal attach process execution.
- Project removal removes Session records and active event logs for the Project's Sessions.
- Prune still archives finished Session event logs.
- Kill remains best-effort for tmux but authoritative for removing the Session record.
- Attach/revive behaviour is consistent and covered by lifecycle tests.
- Existing CLI and TUI behaviour remains stable unless explicitly documented above.
- Full test suite passes with `go test ./...`.
