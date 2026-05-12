# Idle-Nudge Notification Sound Suppression Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Suppress the notification sound when Claude Code fires its ~60-second idle-nudge (`Notification` event arriving after a `Stop`) so users aren't annoyed by a sound that signals nothing actionable.

**Architecture:** Single targeted change in `internal/hooks/handler.go`. Before `applyNormalized` decides to play a sound, it reads the session's *pre-transition* state. If the session was already `Idle` (set by the preceding `Stop` event) and the incoming event is a `Notification`, the sound is suppressed — the state transition to `WaitingForInput` still happens so the TUI shows the visual indicator, but no audio fires.

**Tech Stack:** Go, file-backed state store, `StateStore` interface (already has `Get`).

---

## Background

Claude Code has two distinct notification types that both arrive as `Notification` hooks:

1. **Genuine:** "Approve Bash command?" — Claude is blocked, needs the user's decision (fires from `Running` state)
2. **Idle nudge:** "Claude is waiting for your input" — Claude's internal ~60s timer after a `Stop`; it doesn't know the user is watching cleo

The state flow for an idle nudge:
```
Stop  → EvStop         → Idle           (session finished its turn)
+60s  → EvNotification → WaitingForInput (Claude's timer fires, plays sound ← annoying)
```

The fix: read the "from" state before `Apply` runs. If `fromState == Idle` and the event is `EvNotification`, skip the sound.

**Why read state before `Apply`:** `Apply` mutates the session atomically. If we called `Get` after `Apply`, we'd see `WaitingForInput`, not `Idle` — and couldn't tell which transition just happened.

## Files

- Modify: `internal/hooks/handler.go` — add idle-nudge suppression in `applyNormalized`
- Modify: `internal/hooks/handler_test.go` — add two tests covering idle-nudge and genuine-notification paths

---

### Task 1: Suppress idle-nudge notification sound

**Files:**
- Modify: `internal/hooks/handler.go`
- Modify: `internal/hooks/handler_test.go`

- [ ] **Step 1: Write failing tests**

Add to `internal/hooks/handler_test.go` (after the existing `TestSoundPlaysEvenWhenStateApplyFails` test):

```go
func TestIdleNudgeNotificationDoesNotPlaySound(t *testing.T) {
	deps, st, _ := setup(t)
	player := &recordingPlayer{}
	deps.Sound = player
	_, _ = st.Apply("cleo-x-claude-1", state.EvSessionStart, "")
	_, _ = st.Apply("cleo-x-claude-1", state.EvStop, "")
	// Simulate Claude's ~60s idle nudge arriving after Stop.
	if err := Handle(deps, "claude", "Notification", []byte(`{"message":"Claude is waiting for your input"}`)); err != nil {
		t.Fatal(err)
	}
	if len(player.played) != 0 {
		t.Errorf("idle-nudge Notification (from Idle state) must not play sound, played %v", player.played)
	}
	// State transition to WaitingForInput must still happen for TUI visibility.
	got, _ := st.Get("cleo-x-claude-1")
	if got.State != state.WaitingForInput {
		t.Errorf("state should be WaitingForInput after Notification, got %s", got.State)
	}
}

func TestGenuineNotificationFromRunningPlaysSound(t *testing.T) {
	deps, st, _ := setup(t)
	player := &recordingPlayer{}
	deps.Sound = player
	_, _ = st.Apply("cleo-x-claude-1", state.EvSessionStart, "")
	// Session is Running — Claude needs tool approval (genuine blocking request).
	if err := Handle(deps, "claude", "Notification", []byte(`{"message":"Approve Bash command?"}`)); err != nil {
		t.Fatal(err)
	}
	if len(player.played) != 1 {
		t.Errorf("genuine Notification (from Running state) must play sound, played %v", player.played)
	}
}
```

- [ ] **Step 2: Run failing tests to confirm current behavior**

```bash
cd /Users/dhruvsaxena/Dev/dhruvsaxena1998/cleo
go test ./internal/hooks/ -run "TestIdleNudgeNotificationDoesNotPlaySound|TestGenuineNotificationFromRunningPlaysSound" -v
```

Expected:
- `TestIdleNudgeNotificationDoesNotPlaySound` FAIL — currently the sound plays even from Idle
- `TestGenuineNotificationFromRunningPlaysSound` PASS — genuine notification already plays sound

- [ ] **Step 3: Add idle-nudge suppression to `applyNormalized`**

Replace the `applyNormalized` function in `internal/hooks/handler.go` (currently lines 136–157):

```go
// applyNormalized applies a NormalizedEvent to state, event log, and sound.
func applyNormalized(d Deps, sid string, norm NormalizedEvent) error {
	// Read the pre-transition state so idle-nudge detection can check the
	// "from" state after Apply has already mutated it.
	var fromState state.State
	if d.State != nil {
		if sess, err := d.State.Get(sid); err == nil {
			fromState = sess.State
		}
	}

	var applyErr error
	if !norm.LogOnly && d.State != nil {
		if _, err := d.State.Apply(sid, norm.StateEvent, norm.Message); err != nil {
			applyErr = err
			// continue — still log event and play sound; the agent notified us
		}
	}
	entryType := string(norm.StateEvent)
	if norm.LogType != "" {
		entryType = norm.LogType
	}
	_ = d.Events(sid).Append(events.Entry{
		Type:   entryType,
		Tool:   norm.ToolName,
		Detail: norm.Message,
	})

	// Idle-nudge suppression: a Notification that arrives while the session is
	// already Idle (set by the preceding Stop) is Claude's ~60s internal timer,
	// not a genuine blocking request. Suppress the sound; the state transition to
	// WaitingForInput still happens so the TUI shows the visual indicator.
	idleNudge := norm.StateEvent == state.EvNotification && fromState == state.Idle

	if norm.SoundEvent != "" && d.Config.SoundEventEnabled(norm.SoundEvent) && !sessionFocused(d, sid) && !idleNudge {
		playSound(d, norm.SoundEvent)
	}
	return applyErr
}
```

- [ ] **Step 4: Run all hook tests**

```bash
go test ./internal/hooks/ -v
```

Expected: all PASS, including both new tests.

- [ ] **Step 5: Run full test suite**

```bash
go test ./...
```

Expected: all PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/hooks/handler.go internal/hooks/handler_test.go
git commit -m "fix(hooks): suppress sound for idle-nudge Notifications (Idle→WaitingForInput)"
```
