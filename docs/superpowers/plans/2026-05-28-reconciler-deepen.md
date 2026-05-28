# Reconciler Deepen Implementation Plan

> **For agentic workers:** implement with a red-green-refactor loop. Keep the change focused on splitting decision logic from I/O. Do not opportunistically refactor callers beyond the interface change.

**Goal:** Deepen the Shallow Reconciler by extracting a pure `Decide` function that computes intended state transitions from a snapshot, separating it from the I/O executor that applies them.

**Architecture:** `internal/reconcile` gets a `StateStore` interface, an `Action` type, a pure `Decide` function, an `ApplyActions` executor, and a rewritten `RunOpts` convenience wrapper. Callers (`tui/poll.go`, `cli/ls.go`) update their import/interface usage but keep the same one-liner.

**Tech Stack:** Go, file-backed state store, tmux adapter.

---

## Background

The reconciler (`internal/reconcile/reconcile.go`) is the shallowest module in the codebase. It is a single function `RunOpts` that mixes three concerns in one loop:

1. **Reading state** — calls `st.List()` and `tx.LsPrefix()`
2. **Making decisions** — which sessions need what transitions, and with what event
3. **Mutating state** — calls `st.Apply()` or `st.ApplySynthetic()` inline

The deletion test confirms it: deleting this module forces both callers to inline the loop. The complexity does not vanish.

The current test suite proves the pain: every test creates a real `*state.Store` backed by a temp directory and file lock — even though the decision logic cares only about `[]Session` and `map[string]bool`.

---

## Design decisions

- Extract a **`StateStore` interface** in the reconcile package with `List`, `Apply`, `ApplySynthetic` — exactly the subset `*state.Store` already satisfies.
- Define an **`Action` struct** that encodes one intended state transition: session ID, event, message, and whether to bump `LastEventAt`.
- Extract **`Decide(sessions []state.Session, liveSet map[string]bool, now time.Time, opts Options) []Action`** — a pure function with zero I/O.
- Extract **`ApplyActions(st StateStore, actions []Action) error`** — iterates actions, calls `Apply` or `ApplySynthetic` on the store.
- Keep **`RunOpts`** as a thin wrapper: get live set, list sessions, call `Decide`, call `ApplyActions`.
- The `StateStore` interface lives alongside the existing `TmuxLs` interface, already in the reconcile package.
- `Decide` is exported so tests can call it directly without going through `RunOpts`.

---

## Task 1: Extract interface and types

**Files:**
- `internal/reconcile/reconcile.go`

- [ ] Add `StateStore` interface with `List`, `Apply`, `ApplySynthetic` methods (matching `*state.Store` signatures).
- [ ] Add `Action` struct with `SessionID string`, `Event state.Event`, `Message string`, `BumpTime bool`.
- [ ] Add exported `Decide` function with the pure signature.
- [ ] Add `ApplyActions` function.
- [ ] Rewrite `RunOpts` as a thin wrapper over `Decide` + `ApplyActions`.
- [ ] Remove unused private helpers if any.

---

## Task 2: Add pure-function tests for `Decide`

**Files:**
- `internal/reconcile/reconcile_test.go`

Add tests that call `Decide` directly with `[]state.Session` and `map[string]bool` — no temp dirs, no file locks, no state store:

- [ ] **Missing session marked dead** — session in state but not in live set, not already dead.
- [ ] **Existing dead session not re-dead** — already dead session stays dead even if missing from live set.
- [ ] **Completed session with live tmux revived** — stale completed record with live tmux gets `EvUserResume`.
- [ ] **Completed session with dead tmux stays dead** — completed session not in live set is marked dead (first pass hits dead branch before revive).
- [ ] **Spawning timeout advances to Running** — spawning session older than `SpawningTimeout` gets `EvSessionStart`.
- [ ] **Spawning timeout does not fire early** — spawning session younger than `SpawningTimeout` gets no action.
- [ ] **Idle timeout idle → completed** — idle session past `IdleTimeout` gets `EvIdleTimeout` with `BumpTime: false`.
- [ ] **Idle timeout waiting_for_input → idle** — first idle cycle downgrades to idle, second completes (two-call test).
- [ ] **No action for running sessions** — running session still in live set gets no action.
- [ ] **Multiple sessions produce multiple actions** — one missing, one dead, one revived, one idle-timed-out.

Run tests — they should pass immediately because `Decide` is pure logic extracted from existing `RunOpts`.

---

## Task 3: Keep existing `RunOpts` integration tests

**Files:**
- `internal/reconcile/reconcile_test.go`

The existing `TestReconcileMarksMissingSessionsDead`, `TestReconcileIdleTimeoutPromotesToCompleted`, etc. continue to test the full `RunOpts` path through a real state store. These are now regression tests for the wrapper, not the primary decision-logic tests.

- [ ] Update existing `RunOpts` tests to use `StateStore` interface (they already pass `*state.Store` which satisfies it).
- [ ] Add comment marking them as integration tests for the `RunOpts` wrapper.
- [ ] No behavioural changes — `RunOpts` signature stays `func RunOpts(st StateStore, tx TmuxLs, opts Options) error`.

---

## Task 4: Update callers

**Files:**
- `internal/tui/poll.go`
- `internal/cli/ls.go`

- [ ] In `poll.go`, update `loadStateCmd` — the `c.State` argument is already `*state.Store` which satisfies `StateStore`. No code change needed unless the function signature changed.
- [ ] In `ls.go`, same — no change unless signature changed.
- [ ] Update imports if the reconcile package now exports `StateStore` (unlikely needed by callers).

---

## Task 5: Full regression pass

- [ ] Run `go test ./...`.
- [ ] Verify `Decide` tests run without temp dirs or file locks.
- [ ] Verify existing `RunOpts` tests still pass.

---

## Acceptance criteria

- `Decide` is a pure function: `[]Session + liveSet + now + opts → []Action`. No I/O.
- `ApplyActions` is a simple executor delegating to `StateStore`.
- `RunOpts` is a thin wrapper: get data, call `Decide`, call `ApplyActions`.
- `Decide` tests use plain Go structs — no temp dirs, no file locks.
- Existing `RunOpts` integration tests still pass.
- Full test suite passes.
- No behavioural change for users: reconciler still runs on every poll, same transitions.
- No callers changed beyond signature updates (if any).
