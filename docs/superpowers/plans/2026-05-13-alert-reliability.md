# Alert Reliability Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Fix two root causes of unreliable alert sounds (double-play and silent misses) and add sound-decision logging to make future issues debuggable.

**Architecture:** Three independent changes: (1) make `cleo init` idempotent by checking if the hook command string already exists before writing, (2) reduce focus TTL from 30 min to 5 min, (3) log the sound suppression reason to `hook-trace.log` on every hook event.

**Tech Stack:** Go, `encoding/json`, `internal/hooks`, `internal/focus`

---

## File Map

| File | Change |
|---|---|
| `internal/hooks/install.go` | Add `hookCommandPresent` helper; skip event install if command already present |
| `internal/hooks/install_test.go` | Add idempotency test for `InstallClaude` and `InstallCodex` |
| `internal/focus/store.go` | Change `focusTTL` constant from 30 min → 5 min |
| `internal/focus/store_test.go` | Add test verifying TTL boundary at 5 min |
| `internal/hooks/handler.go` | Add `soundDecision` type + `logSoundDecision`; restructure sound block in `applyNormalized` |
| `internal/hooks/handler_test.go` | Add tests verifying each sound suppression reason is logged |

---

## Task 1: Hook install idempotency

**Files:**
- Modify: `internal/hooks/install.go`
- Modify: `internal/hooks/install_test.go`

The problem: `InstallClaude` uses `equalsHook` (whole-JSON comparison) to detect existing entries. If Claude Code adds a `matcher` field or any extra key, `equalsHook` returns false → conflict error on re-run. Even without that, there is no command-level dedup: a hook entry could exist with the right shape but different fields and still fire the cleo binary twice if installed via two paths.

The fix: add `hookCommandPresent(entry any, cmd string) bool` that searches the nested `[{hooks: [{command: "..."}]}]` structure for an exact command string. If found, skip that event entirely — no write, no error.

- [ ] **Step 1: Write the failing test**

Add to `internal/hooks/install_test.go`:

```go
func TestInstallClaudeIdempotent(t *testing.T) {
	dir := t.TempDir()
	settingsPath := filepath.Join(dir, "settings.json")
	_ = os.WriteFile(settingsPath, []byte("{}"), 0o644)

	// First install
	if err := InstallClaude(settingsPath, "/usr/local/bin/cleo", false); err != nil {
		t.Fatalf("first install: %v", err)
	}
	b1, _ := os.ReadFile(settingsPath)

	// Second install — must not error and must produce identical output
	if err := InstallClaude(settingsPath, "/usr/local/bin/cleo", false); err != nil {
		t.Fatalf("second install: %v", err)
	}
	b2, _ := os.ReadFile(settingsPath)

	if string(b1) != string(b2) {
		t.Errorf("second install mutated settings.json:\nbefore: %s\nafter:  %s", b1, b2)
	}
}

func TestInstallCodexIdempotent(t *testing.T) {
	dir := t.TempDir()
	hooksPath := filepath.Join(dir, "hooks.json")
	configPath := filepath.Join(dir, "config.toml")

	if err := InstallCodex(hooksPath, configPath, "/usr/local/bin/cleo", false); err != nil {
		t.Fatalf("first install: %v", err)
	}
	b1, _ := os.ReadFile(hooksPath)

	if err := InstallCodex(hooksPath, configPath, "/usr/local/bin/cleo", false); err != nil {
		t.Fatalf("second install: %v", err)
	}
	b2, _ := os.ReadFile(hooksPath)

	if string(b1) != string(b2) {
		t.Errorf("second install mutated hooks.json:\nbefore: %s\nafter:  %s", b1, b2)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
go test ./internal/hooks/... -run "TestInstallClaudeIdempotent|TestInstallCodexIdempotent" -v
```

Expected: FAIL — second install currently returns an error on equal entries (or silently overwrites with the same bytes, so this may actually pass for identical binary paths; run to confirm current behavior before proceeding).

- [ ] **Step 3: Add `hookCommandPresent` helper to `internal/hooks/install.go`**

Add this function at the bottom of `install.go`, before `equalsHook`:

```go
// hookCommandPresent reports whether cmd already appears as a command string
// in any hook inside the existing event entry. The entry is the value stored
// at hooks["EventName"] — a []any of hook-group objects, each with a "hooks"
// []any of individual hook maps.
func hookCommandPresent(entry any, cmd string) bool {
	groups, ok := entry.([]any)
	if !ok {
		return false
	}
	for _, rawGroup := range groups {
		group, ok := rawGroup.(map[string]any)
		if !ok {
			continue
		}
		rawHooks, ok := group["hooks"].([]any)
		if !ok {
			continue
		}
		for _, rawHook := range rawHooks {
			h, ok := rawHook.(map[string]any)
			if !ok {
				continue
			}
			if c, _ := h["command"].(string); c == cmd {
				return true
			}
		}
	}
	return false
}
```

- [ ] **Step 4: Update the install loop in `InstallClaude`**

Replace the existing event loop inside `InstallClaude`:

```go
// existing code to replace:
for _, ev := range claudeEvents {
    want := expected[ev]
    if existing, ok := hooks[ev]; ok {
        if !equalsHook(existing, want) && !force {
            return fmt.Errorf("conflict: %s already has a different hook (re-run with --force to overwrite)", ev)
        }
    }
    hooks[ev] = want
}
```

Replace with:

```go
for _, ev := range claudeEvents {
    want := expected[ev]
    cmd := fmt.Sprintf("%s hook claude %s", cleoBin, ev)
    if hookCommandPresent(hooks[ev], cmd) {
        continue // already installed — skip, don't overwrite
    }
    if existing, ok := hooks[ev]; ok {
        if !equalsHook(existing, want) && !force {
            return fmt.Errorf("conflict: %s already has a different hook (re-run with --force to overwrite)", ev)
        }
    }
    hooks[ev] = want
}
```

- [ ] **Step 5: Update the install loop in `InstallCodex`**

Same change in `InstallCodex`. Replace:

```go
for _, ev := range codexEvents {
    want := expected[ev]
    if existing, ok := hooks[ev]; ok {
        if !equalsHook(existing, want) && !force {
            return fmt.Errorf("conflict: %s already has a different hook (re-run with --force to overwrite)", ev)
        }
    }
    hooks[ev] = want
}
```

With:

```go
for _, ev := range codexEvents {
    want := expected[ev]
    cmd := fmt.Sprintf("%s hook codex %s", cleoBin, ev)
    if hookCommandPresent(hooks[ev], cmd) {
        continue // already installed — skip
    }
    if existing, ok := hooks[ev]; ok {
        if !equalsHook(existing, want) && !force {
            return fmt.Errorf("conflict: %s already has a different hook (re-run with --force to overwrite)", ev)
        }
    }
    hooks[ev] = want
}
```

- [ ] **Step 6: Run all hook tests**

```bash
go test ./internal/hooks/... -v
```

Expected: all PASS including the two new idempotency tests.

- [ ] **Step 7: Commit**

```bash
git add internal/hooks/install.go internal/hooks/install_test.go
git commit -m "fix(hooks): make cleo init idempotent — skip events where command already present"
```

---

## Task 2: Tighten focus TTL from 30 min to 5 min

**Files:**
- Modify: `internal/focus/store.go`
- Modify: `internal/focus/store_test.go`

The problem: if cleo crashes or the tmux focus hook misfires, `focus.json` keeps the session marked as focused for up to 30 minutes. All sounds during that window are suppressed.

The fix: reduce the TTL to 5 minutes. 5 minutes survives normal tmux detach/reattach cycles (which take seconds) while limiting the false-positive suppression window from 30 min to 5 min.

- [ ] **Step 1: Write the failing test**

Add to `internal/focus/store_test.go`:

```go
func TestFocusTTLIsUnder10Minutes(t *testing.T) {
	store := NewStore(filepath.Join(t.TempDir(), "focus.json"))

	// Simulate a session that was focused 6 minutes ago
	staleTime := time.Now().Add(-6 * time.Minute)
	f := fileFormat{
		Sessions: map[string]sessionFocus{
			"cleo-app-claude-1": {Focused: true, UpdatedAt: staleTime},
		},
	}
	b, _ := json.MarshalIndent(f, "", "  ")
	_ = os.WriteFile(store.path, b, 0o644)

	if store.IsFocused("cleo-app-claude-1") {
		t.Error("focused=true with UpdatedAt 6 min ago should be treated as stale (TTL must be ≤ 5 min)")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
go test ./internal/focus/... -run TestFocusTTLIsUnder10Minutes -v
```

Expected: FAIL — current TTL is 30 min so a 6-minute-old entry is still considered fresh.

- [ ] **Step 3: Change the TTL constant**

In `internal/focus/store.go`, change:

```go
const focusTTL = 30 * time.Minute
```

to:

```go
const focusTTL = 5 * time.Minute
```

- [ ] **Step 4: Run all focus tests**

```bash
go test ./internal/focus/... -v
```

Expected: all PASS. The existing `TestIsFocusedReturnsFalseWhenStale` uses a 2-hour stale time which still exceeds 5 min, so it remains green.

- [ ] **Step 5: Commit**

```bash
git add internal/focus/store.go internal/focus/store_test.go
git commit -m "fix(focus): reduce TTL from 30 min to 5 min to limit false-positive sound suppression"
```

---

## Task 3: Log sound decision to hook-trace.log

**Files:**
- Modify: `internal/hooks/handler.go`
- Modify: `internal/hooks/handler_test.go`

The problem: when a sound is suppressed or played, there is no record of why. Intermittent issues (sound plays twice, sound never plays) are impossible to debug without reproducing.

The fix: after every hook event that carries a sound event, append a JSON line to `hook-trace.log` with the session ID, sound event name, and reason (`played`, `focus`, `idle-nudge`, `disabled`).

- [ ] **Step 1: Write the failing tests**

Add to `internal/hooks/handler_test.go`:

```go
func readTraceLines(t *testing.T, p paths.Paths) []map[string]any {
	t.Helper()
	b, err := os.ReadFile(p.HookTraceLog())
	if err != nil {
		return nil
	}
	var out []map[string]any
	for _, line := range strings.Split(strings.TrimSpace(string(b)), "\n") {
		if line == "" {
			continue
		}
		var m map[string]any
		if err := json.Unmarshal([]byte(line), &m); err != nil {
			t.Fatalf("bad trace line: %v\nline: %s", err, line)
		}
		out = append(out, m)
	}
	return out
}

func soundTraceLines(lines []map[string]any) []map[string]any {
	var out []map[string]any
	for _, l := range lines {
		if _, ok := l["sound_event"]; ok {
			out = append(out, l)
		}
	}
	return out
}

func TestSoundDecisionLoggedAsPlayed(t *testing.T) {
	deps, st, p := setup(t)
	player := &recordingPlayer{}
	deps.Sound = player
	_, _ = st.Apply("cleo-x-claude-1", state.EvSessionStart, "")

	if err := Handle(deps, "claude", "SessionEnd", []byte(`{}`)); err != nil {
		t.Fatal(err)
	}

	lines := soundTraceLines(readTraceLines(t, p))
	if len(lines) == 0 {
		t.Fatal("expected a sound decision trace line")
	}
	last := lines[len(lines)-1]
	if reason, _ := last["reason"].(string); reason != "played" {
		t.Errorf("expected reason=played, got %q", reason)
	}
	if se, _ := last["sound_event"].(string); se == "" {
		t.Errorf("expected sound_event to be set")
	}
}

func TestSoundDecisionLoggedAsFocus(t *testing.T) {
	deps, st, p := setup(t)
	player := &recordingPlayer{}
	deps.Sound = player
	deps.Focused = func(sid string) bool { return sid == "cleo-x-claude-1" }
	_, _ = st.Apply("cleo-x-claude-1", state.EvSessionStart, "")

	if err := Handle(deps, "claude", "SessionEnd", []byte(`{}`)); err != nil {
		t.Fatal(err)
	}

	lines := soundTraceLines(readTraceLines(t, p))
	if len(lines) == 0 {
		t.Fatal("expected a sound decision trace line")
	}
	last := lines[len(lines)-1]
	if reason, _ := last["reason"].(string); reason != "focus" {
		t.Errorf("expected reason=focus, got %q", reason)
	}
}

func TestSoundDecisionLoggedAsDisabled(t *testing.T) {
	deps, st, p := setup(t)
	player := &recordingPlayer{}
	deps.Sound = player
	deps.Config.Sound.EventEnabled = map[string]bool{"session_completed": false}
	_, _ = st.Apply("cleo-x-claude-1", state.EvSessionStart, "")

	if err := Handle(deps, "claude", "SessionEnd", []byte(`{}`)); err != nil {
		t.Fatal(err)
	}

	lines := soundTraceLines(readTraceLines(t, p))
	if len(lines) == 0 {
		t.Fatal("expected a sound decision trace line")
	}
	last := lines[len(lines)-1]
	if reason, _ := last["reason"].(string); reason != "disabled" {
		t.Errorf("expected reason=disabled, got %q", reason)
	}
}

func TestSoundDecisionLoggedAsIdleNudge(t *testing.T) {
	deps, st, p := setup(t)
	player := &recordingPlayer{}
	deps.Sound = player
	// Put session in Idle state so the Notification is treated as an idle-nudge
	_, _ = st.Apply("cleo-x-claude-1", state.EvSessionStart, "")
	_, _ = st.Apply("cleo-x-claude-1", state.EvStop, "")

	if err := Handle(deps, "claude", "Notification", []byte(`{"message":"nudge"}`)); err != nil {
		t.Fatal(err)
	}

	lines := soundTraceLines(readTraceLines(t, p))
	if len(lines) == 0 {
		t.Fatal("expected a sound decision trace line")
	}
	last := lines[len(lines)-1]
	if reason, _ := last["reason"].(string); reason != "idle-nudge" {
		t.Errorf("expected reason=idle-nudge, got %q", reason)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
go test ./internal/hooks/... -run "TestSoundDecision" -v
```

Expected: all FAIL — no sound decision lines are written to the trace log yet.

- [ ] **Step 3: Add `soundDecision` type and `logSoundDecision` to `internal/hooks/handler.go`**

Add these after the `hookTrace` struct (around line 140):

```go
type soundDecision struct {
	SessionID  string `json:"session_id"`
	SoundEvent string `json:"sound_event"`
	Reason     string `json:"reason"` // played | focus | idle-nudge | disabled
}

func logSoundDecision(p paths.Paths, d soundDecision) {
	f, err := os.OpenFile(p.HookTraceLog(), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	defer f.Close()
	row := struct {
		At string `json:"at"`
		soundDecision
	}{
		At:            time.Now().Format(time.RFC3339),
		soundDecision: d,
	}
	b, _ := json.Marshal(row)
	fmt.Fprintln(f, string(b))
}
```

- [ ] **Step 4: Restructure the sound block in `applyNormalized`**

Replace the existing sound-play block in `applyNormalized`:

```go
// existing code to replace:
if norm.SoundEvent != "" && d.Config.SoundEventEnabled(norm.SoundEvent) && !sessionFocused(d, sid) && !idleNudge {
    playSound(d, norm.SoundEvent)
}
```

Replace with:

```go
if norm.SoundEvent != "" {
    var reason string
    switch {
    case !d.Config.SoundEventEnabled(norm.SoundEvent):
        reason = "disabled"
    case sessionFocused(d, sid):
        reason = "focus"
    case idleNudge:
        reason = "idle-nudge"
    default:
        reason = "played"
        playSound(d, norm.SoundEvent)
    }
    logSoundDecision(d.Paths, soundDecision{
        SessionID:  sid,
        SoundEvent: norm.SoundEvent,
        Reason:     reason,
    })
}
```

- [ ] **Step 5: Run all hook tests**

```bash
go test ./internal/hooks/... -v
```

Expected: all PASS including the four new `TestSoundDecision*` tests.

- [ ] **Step 6: Run full test suite**

```bash
go test ./...
```

Expected: all PASS.

- [ ] **Step 7: Commit**

```bash
git add internal/hooks/handler.go internal/hooks/handler_test.go
git commit -m "fix(hooks): log sound decision (played/focus/idle-nudge/disabled) to hook-trace.log"
```

---

## Acceptance criteria cross-check

| Criterion | Task |
|---|---|
| Running `cleo init` twice produces no duplicate entries | Task 1 — `hookCommandPresent` skips already-present commands |
| Sound event fired while focus TTL expired (5+ min) plays | Task 2 — TTL reduced to 5 min |
| `hook-trace.log` records sound decision reason for every hook event | Task 3 — `logSoundDecision` called from `applyNormalized` |
| Existing suppression behavior preserved (focused session, disabled sound) | Task 3 — same conditions, now also logged |
