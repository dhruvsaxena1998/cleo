# State Machine Hardening Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Prevent hook events from resurrecting terminal-state sessions (Dead, Completed, Errored), which currently causes sessions to appear active in the TUI when they cannot be attached to.

**Architecture:** Single change in `internal/state/transitions.go` — guard the `NextState` function so that sessions in terminal states ignore all hook events except `EvDead` (which is idempotent). One new test block in the existing `state_test.go`.

**Tech Stack:** Go, file-backed state store.

---

## Background

**EC-4:** `NextState` has no guard on the `from` state. Hook events dispatched after a session terminates (delayed delivery, process scheduling, reconciler race) can resurrect it:

```
NextState(Dead, EvNotification) → WaitingForInput   ← broken
NextState(Dead, EvStop)        → Idle               ← broken
NextState(Dead, EvSessionStart)→ Running            ← broken
NextState(Completed, EvNotification) → WaitingForInput  ← broken
```

A resurrected session shows as "waiting for input" in the TUI, but `HasSession` returns false (tmux session is gone). The session is permanently stuck — the reconciler will not re-mark it Dead because `EvDead` only fires when the session disappears from `tmux ls`, which it already has. The only escape is `cleo prune`.

The fix is a guard at the top of `NextState`: if `from.IsFinished()` and the event is not `EvDead`, return `from` unchanged. `EvDead` is still allowed through because it is idempotent (`Dead → Dead`) and the reconciler's guard (`s.State != state.Dead`) already prevents redundant applications.

## Files

- Modify: `internal/state/transitions.go` — add terminal-state guard
- Modify: `internal/state/state_test.go` — add resurrection test cases

---

### Task 1: Guard terminal states in `NextState`

**Files:**
- Modify: `internal/state/transitions.go`
- Modify: `internal/state/state_test.go`

- [ ] **Step 1: Write failing tests**

Add to `TestNextState` in `internal/state/state_test.go` — extend the `cases` slice:

```go
// Terminal states must not be resurrected by hook events.
{Dead, EvNotification, Dead},
{Dead, EvStop, Dead},
{Dead, EvSessionStart, Dead},
{Dead, EvUserResume, Dead},
{Dead, EvPreToolUse, Dead},
{Dead, EvPostToolUse, Dead},
{Dead, EvSessionEnd, Dead},
{Completed, EvNotification, Completed},
{Completed, EvStop, Completed},
{Completed, EvSessionStart, Completed},
{Errored, EvNotification, Errored},
{Errored, EvStop, Errored},
// EvDead is still allowed — idempotent.
{Dead, EvDead, Dead},
{Completed, EvDead, Dead},
{Errored, EvDead, Dead},
```

- [ ] **Step 2: Run tests to confirm they fail**

```bash
cd /Users/dhruvsaxena/Dev/dhruvsaxena1998/cleo
go test ./internal/state/ -run TestNextState -v
```

Expected: multiple FAIL lines like `NextState(dead, notification) = waiting_for_input, want dead`.

- [ ] **Step 3: Add the guard to `NextState`**

In `internal/state/transitions.go`, add a guard as the first statement of the function:

```go
func NextState(from State, ev Event) State {
	// Terminal states ignore hook events. Only EvDead can still transition
	// (Dead absorbs all terminal states; the reconciler guards against
	// redundant EvDead applications separately).
	if from.IsFinished() && ev != EvDead {
		return from
	}
	switch ev {
	case EvDead:
		return Dead
	case EvError:
		return Errored
	case EvSessionEnd:
		return Completed
	case EvIdleTimeout:
		if from == WaitingForInput {
			return Idle
		}
		if from == Idle {
			return Completed
		}
		return from
	case EvSessionStart:
		return Running
	case EvNotification:
		return WaitingForInput
	case EvStop:
		return Idle
	case EvUserResume:
		return Running
	case EvPreToolUse, EvPostToolUse:
		if from == Spawning {
			return Running
		}
		if from == WaitingForInput {
			return Running
		}
		if from == Idle {
			return Running
		}
		return from
	}
	return from
}
```

- [ ] **Step 4: Run all state tests**

```bash
go test ./internal/state/ -v
```

Expected: all PASS.

- [ ] **Step 5: Run full test suite**

```bash
go test ./...
```

Expected: all PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/state/transitions.go internal/state/state_test.go
git commit -m "fix(state): terminal states (Dead/Completed/Errored) ignore hook events to prevent resurrection"
```
