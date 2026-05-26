# Session Lifecycle Module PRD

**Date:** 2026-05-26  
**Status:** Ready for agent  
**Scope:** Creation tracer-bullet for the deeper Session lifecycle module

## Problem Statement

Cleo has Session lifecycle knowledge duplicated across the CLI and TUI. Creating a Session currently requires each caller to understand how to resolve or register a Project, choose a Session name, write the initial Session state, start tmux, install focus tracking, bind the detach key, and roll back state when tmux launch fails.

This makes the Session lifecycle shallow: the interface exposed to each caller is nearly as complex as the implementation. Every future change to Session creation risks drifting between CLI and TUI. It also makes the upcoming broader lifecycle work harder to test because the real behaviour is spread across callers instead of concentrated behind one seam.

## Solution

Introduce a deep Session lifecycle module and route Session creation through it as a tracer-bullet.

From the user's perspective, `cleo run` and TUI Session spawning should behave the same as today. The change is architectural: Project resolution/registration mechanics, Session naming, state-first launch, tmux startup, rollback, focus hook installation, and detach-key binding become one reusable implementation.

This tracer-bullet proves the seam with Session creation first. Later work can move attach, revive, kill, prune, rename, and Project removal into the same Session lifecycle module without one giant diff.

## User Stories

1. As a Cleo user, I want `cleo run` to keep spawning Sessions as before, so that this refactor does not change my workflow.
2. As a Cleo user, I want the TUI new-Session flow to keep spawning Sessions as before, so that this refactor does not change interactive use.
3. As a Cleo user, I want a Session created from an unregistered directory to still be registerable, so that first-run flows remain smooth.
4. As a Cleo user, I want `cleo run` to keep asking before registering a new Project, so that Cleo does not register directories without consent.
5. As a TUI user, I want the popup-driven new-Project flow to keep registering the Project only after I submit it, so that UI consent stays explicit.
6. As a Cleo user, I want Session names to continue being slugified, so that Session IDs remain predictable.
7. As a Cleo user, I want duplicate requested Session names to be deduped, so that new Sessions do not collide with existing Sessions.
8. As a Cleo user, I want generated Session names to remain unique within a Project and agent, so that multiple Sessions can run in parallel.
9. As a Cleo user, I want a new Session to appear as `spawning` before hooks arrive, so that startup attribution still works.
10. As a Cleo user, I want hook events during launch to resolve to the new Session, so that Session state moves out of `spawning` correctly.
11. As a Cleo user, I want failed tmux launches to remove the partial Session record, so that `cleo ls` does not show a Session that never started.
12. As a Cleo user, I want tmux launch failures to remain visible to the caller, so that CLI and TUI can report spawn failures.
13. As a Cleo user, I want focus tracking to keep working after spawning a Session, so that notification sound suppression still respects focused Sessions.
14. As a Cleo user, I want the configured tmux detach key to keep working after spawning a Session, so that tmux usability does not regress.
15. As a Cleo user, I want `--no-attach` to keep spawning without attaching, so that automation flows keep working.
16. As a Cleo user, I want default attach after `cleo run` to keep working, so that command-line launching still drops me into the Session.
17. As a Cleo user, I want TUI spawning to refresh state after creating a Session, so that the new Session appears without restarting Cleo.
18. As a Cleo user, I want failed TUI spawning to leave me in a clear state with an error message, so that I can correct the input and retry.
19. As a Cleo maintainer, I want Session creation rules in one module, so that Project, Session, state, tmux, focus, and detach-key behaviour do not drift.
20. As a Cleo maintainer, I want the Session lifecycle module to be deep, so that callers learn a small interface while the implementation absorbs the launch details.
21. As a Cleo maintainer, I want the interface to return typed outcomes instead of caller-specific text, so that CLI and TUI can render differently without duplicating lifecycle rules.
22. As a Cleo maintainer, I want state-first creation preserved, so that hook attribution continues to work during launch.
23. As a Cleo maintainer, I want rollback to be part of the Session lifecycle implementation, so that cleanup after launch failure is tested once.
24. As a Cleo maintainer, I want Project registration mechanics inside the lifecycle module, so that Project/Session coupling is local.
25. As a Cleo maintainer, I want consent and prompts outside the lifecycle module, so that CLI and TUI keep owning user interaction.
26. As a Cleo maintainer, I want Worktree behaviour excluded from this change, so that a not-yet-implemented feature does not distort the seam.
27. As a Cleo maintainer, I want tests around lifecycle behaviour instead of testing private helper extraction, so that the test surface matches the interface.
28. As a Cleo maintainer, I want existing CLI and TUI tests to keep passing, so that external behaviour stays stable.
29. As a future Cleo maintainer, I want the Session lifecycle term documented, so that future architecture reviews use the same domain language.
30. As a future agent, I want the creation tracer-bullet to be small and complete, so that later lifecycle migration can proceed safely.

## Implementation Decisions

- Build a new deep module named after the documented domain term: Session lifecycle.
- The initial implementation migrates Session creation only. Attach, revive, kill, prune, rename, and Project removal remain future migrations.
- Session lifecycle scope is the full concept, but the first deliverable is a creation tracer-bullet.
- Session lifecycle owns Project resolution/registration mechanics.
- Callers own user consent, prompts, popups, status text, and rendering.
- Session creation remains state-first: write a `spawning` Session before starting tmux.
- If tmux launch fails, Session lifecycle rolls back the Session record and returns a failure outcome.
- If the process dies after state write but before tmux launch completes, existing reconciliation can mark the Session dead.
- Session lifecycle owns Session name slugging/deduplication for creation.
- Session lifecycle owns launch side effects for focus hook installation and detach-key binding.
- Focus hook installation and detach-key binding are best-effort side effects unless there is already a hard failure creating the Session.
- Worktree behaviour is explicitly out of scope until the Worktree feature is actively planned.
- The tmux dependency remains the real external seam and should be testable through an adapter.
- Store modules remain concrete file-backed modules for now; do not introduce store abstractions unless tests prove they are needed.
- Clock and name generation may be injectable internal seams for deterministic tests.
- The module returns typed outcomes/categories that callers convert into CLI output or TUI status.
- No schema migration is required.
- No config format change is required.
- No command-line flag change is required.
- The `Session lifecycle` term has been added to the project glossary.

## Testing Decisions

- Good tests exercise external lifecycle behaviour: inputs, returned outcomes, written Session state, tmux calls, rollback, and caller-visible behaviour.
- Avoid tests that assert private helper structure or internal function ordering except where ordering is an externally important invariant.
- Add focused tests for the Session lifecycle creation module.
- Test that creation returns a needs-registration outcome when a path is not registered and auto-registration is not allowed.
- Test that creation registers a Project when auto-registration is allowed.
- Test that requested names are slugified and deduped.
- Test that generated names avoid existing Session names using deterministic name generation.
- Test state-first launch by using a tmux fake that verifies the Session record exists during tmux startup.
- Test rollback by making tmux launch fail and asserting the Session record is removed.
- Test that tmux launch receives the resolved Project path, configured agent command, Session ID name, and `CLEO_SESSION_ID` environment value.
- Test that focus hook installation is attempted for tmux adapters that support it.
- Test that detach-key binding is attempted when configured.
- Test that existing CLI `run` behaviour remains stable using current command tests as prior art.
- Test that existing TUI spawn behaviour remains stable using current TUI tests as prior art.
- Run the full Go test suite after migration.

## Out of Scope

- Implementing Worktree creation or cleanup.
- Moving attach behaviour into Session lifecycle.
- Moving revive/mark-dead behaviour into Session lifecycle.
- Moving kill behaviour into Session lifecycle.
- Moving prune behaviour into Session lifecycle.
- Moving rename behaviour into Session lifecycle.
- Moving Project removal into Session lifecycle.
- Changing the Session state machine.
- Changing hook protocol normalization.
- Changing tmux naming conventions.
- Changing config schema.
- Changing CLI flags or TUI keybindings.
- Changing event log retention policy in this tracer-bullet.

## Further Notes

- The architecture review's top recommendation was to deepen Session operations first because Session creation is Cleo's hot path and the duplication already spans CLI and TUI.
- During design, the selected direction was a Session lifecycle module, not a spawn-only module.
- During design, state-first creation was preserved for hook attribution.
- During design, Project registration mechanics were placed inside Session lifecycle while caller consent stayed outside.
- During design, Worktree was deferred because it is not implemented yet.
- During design, focus hook installation and detach-key binding were kept inside lifecycle launch side effects.
- During design, typed outcomes were preferred over plain errors so CLI and TUI can render independently.
- During design, minimal seams were preferred: tmux is the real adapter seam; stores remain concrete for now.
