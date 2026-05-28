# Reconciler Deepen PRD

**Date:** 2026-05-28  
**Status:** Ready for agent  
**Scope:** Deepen the Shallow Reconciler by separating decision logic from I/O

## Problem Statement

The reconciler (`internal/reconcile`) is the shallowest module in Cleo. It is a single function that mixes three concerns in one loop: reading state from disk, deciding which sessions need what state transitions, and writing those transitions back. This makes the decision logic untestable in isolation — every test today requires a real JSON-backed state store on disk, with temp directories and file locks, even though the decision tree cares only about a slice of sessions and a map of live tmux sessions.

This shallowness creates real friction: a bug in idle-timeout promotion or spawning-timeout detection requires tracing through mixed reads, writes, and time comparisons in the same loop. Extracting a pure decision function would let the entire state-machine logic be tested with plain Go structs and no I/O, making edge cases easy to pin down with regression tests.

## Solution

Split the reconciler into a pure `Decide` function that computes intended state transitions from a snapshot, and an `ApplyActions` executor that applies them through a `StateStore` interface. The existing `RunOpts` becomes a thin wrapper over these two. The change is architectural — user-facing behaviour does not change at all.

From the developer's perspective, `Decide` takes `[]Session`, `map[string]bool` (live tmux sessions), `time.Time`, and `Options` and returns `[]Action`. Testing it requires nothing but input structs and assertions on output structs. No temp files, no locks, no tmux mocks.

## User Stories

1. As a Cleo user, I want the reconciler to keep running on every TUI poll, so that dead and timed-out sessions are still detected automatically.
2. As a Cleo user, I want `cleo ls` to still reconcile state, so that the listing is always current.
3. As a Cleo user, I want idle sessions to still be promoted to completed after the configured timeout, so that the dashboard stays tidy.
4. As a Cleo user, I want waiting-for-input sessions to still progress through idle to completed across two reconcile cycles, so that stuck sessions are eventually cleaned up.
5. As a Cleo user, I want spawning sessions past the timeout to still advance to running, so that the dashboard reflects that the agent started.
6. As a Cleo user, I want dead tmux sessions to still be marked dead on reconcile, so that `cleo ls` and the dashboard do not show ghost sessions.
7. As a Cleo user, I want completed sessions whose tmux session is still alive to be revived to idle, so that re-attaching works without stale-done records.
8. As a Cleo maintainer, I want the reconciler's decision logic to be testable without disk I/O, so that edge cases can be pinned with simple data-driven tests.
9. As a Cleo maintainer, I want a `StateStore` interface in the reconcile package, so that tests can use in-memory fakes instead of real JSON files.
10. As a Cleo maintainer, I want the existing `RunOpts` tests to keep passing, so that the wrapper path is regression-tested.
11. As a Cleo maintainer, I want no behavioural change from this refactor, so that I can ship it without manual QA.
12. As a future Cleo developer, I want to see the state-machine decision tree in one pure function, so that I can understand timeout and transition rules without tracing through I/O.

## Implementation Decisions

- The new `StateStore` interface lives in the reconcile package with `List`, `Apply`, and `ApplySynthetic` methods — exactly matching `*state.Store`.
- The `Action` struct encodes one intended transition: `SessionID`, `Event`, `Message`, `BumpTime` (controls Apply vs ApplySynthetic).
- `Decide` is exported so tests can call it directly.
- `ApplyActions` is exported for use by `RunOpts` and potentially future callers.
- `RunOpts` keeps its existing signature (`func RunOpts(st StateStore, tx TmuxLs, opts Options) error`) so callers need no changes — `*state.Store` already satisfies `StateStore`.
- The existing `TmuxLs` interface stays as-is.
- No changes to `state.Session`, `state.Event`, or the transition table in `state/transitions.go`.
- No changes to the tmux adapter.
- No config schema changes.
- No CLI flag or TUI keybinding changes.
- The `Decide` function is deterministic for a given input triplet (sessions, liveSet, now).

## Testing Decisions

- Good tests exercise the decision logic: given a set of sessions and a live set, what actions are produced?
- `Decide` tests use plain Go slices and maps — no temp dirs, no file locks, no state store.
- Each branch of the decision tree gets at least one test:
  - missing session not dead → marked dead
  - already dead session → no re-dead
  - completed with live tmux → revived (EvUserResume, BumpTime: true)
  - completed without live tmux → marked dead
  - spawning before timeout → no action
  - spawning after timeout → advanced to Running (BumpTime: true)
  - idle before timeout → no action
  - idle after timeout → EvIdleTimeout (BumpTime: false)
  - waiting_for_input before timeout → no action
  - waiting_for_input after timeout → EvIdleTimeout (BumpTime: false)
  - two-cycle waiting_for_input → idle then completed
  - running session with live tmux → no action
- Existing `RunOpts` integration tests remain as regression tests for the wrapper path — they use `*state.Store` with temp dirs, which still works because `*state.Store` satisfies `StateStore`.
- Prior art: the existing `reconcile_test.go` file has 6 test functions using fake tmux and temp-dir state stores. The new pure tests sit alongside them.

## Out of Scope

- Changing the session state machine or transition table.
- Changing tmux adapter behaviour.
- Changing the reconciler's `Options` struct.
- Adding new reconciler features (e.g. new transition types).
- Refactoring callers beyond interface compatibility.
- Changing the `state.Store` implementation.
- Changing event log or archive behaviour.
- Touching the sessionlifecycle module.
- Touching hooks or focus modules.

## Further Notes

- This candidate was the top recommendation from the 2026-05-28 architecture review.
- During the architecture review, the reconciler was identified as shallow because it fails the deletion test (deleting it forces complexity into callers) and its interface is nearly as complex as its implementation.
- The deepen preserves the one-adapter rule: the `StateStore` interface is used by exactly one concrete type (`*state.Store`), but the test seam is the primary motive — not anticipation of a second adapter.
- The `BumpTime` field on `Action` encodes the reconciler's knowledge about which transitions should reset the idle clock (Apply) and which should not (ApplySynthetic). This was previously implicit in whether the code called `st.Apply` or `st.ApplySynthetic`.
