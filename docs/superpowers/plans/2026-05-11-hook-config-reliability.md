# Hook & Config Reliability Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Fix four bugs: hook timeout too short causing dropped notifications under load; stale `CLEO_SESSION_ID` silently dropping Claude notifications; state failure suppressing sounds; and `mergeDefaults` silently disabling sound and overwriting UI settings.

**Architecture:** Four independent fixes across three packages. Each task is safe to apply independently. No interface changes except `Sound.Enabled` becoming `*bool` (backward-compatible via `mergeDefaults`).

**Tech Stack:** Go, TOML config, file lock (`flock`), hooks protocol.

---

## Background

### EC-3: 2-second hook timeout causes dropped notifications under lock contention
`ExpectedClaudeEntries` and `ExpectedCodexEntries` install hooks with `"timeout": 2` seconds. The TUI runs `reconcile.RunOpts` every 750ms, which holds the write lock on the state file. With rapid tool calls, multiple `cleo hook` processes queue on the lock. The last process in the queue may time out before it can write. Claude kills the hook — state not updated, sound not played, no error visible. Fix: raise to 5 seconds.

### EC-5: Stale `CLEO_SESSION_ID` silently drops Claude notifications
When a Claude session is pruned from the state store while the Claude process is still running, every hook fires with a valid-looking `CLEO_SESSION_ID` that `state.Get()` returns `ErrSessionNotFound` for. Claude's `UsesCwdFallback() = false` means the CWD fallback is never attempted. The event is dropped silently — logged to `hook-trace.log` with `fallback_reason: env_unknown_session`, but not to `hook-errors.log`. Fix: when the env var is set but the session is unknown, attempt CWD fallback regardless of protocol.

### EC-7: `state.Apply` failure atomically drops event log entry AND sound
`applyNormalized` returns immediately when `state.Apply` errors (disk full, lock timeout, corrupt state file). The event log write and sound play are skipped. The agent DID notify; the user should hear the sound even if the state file is temporarily unwritable. Fix: continue with event log and sound on state error; return the error at the end.

### EC-9: `mergeDefaults` never fills in `Sound.Enabled`
`Sound.Enabled` is a `bool`. Go's zero value for `bool` is `false`. `mergeDefaults` fills in `Sound.Volume`, `Sound.Events`, and `Sound.EventEnabled` — but not `Sound.Enabled`. A config file with a `[sound]` section that lacks `enabled = true` (e.g., pre-dates the sound feature, or was manually edited) loads with `Enabled = false`, silently disabling all sounds. Fix: change `Sound.Enabled` to `*bool` so `nil` (absent) is distinguishable from `false` (explicit).

### EC-10: `mergeDefaults` replaces entire UI config when `SidebarWidth == 0`
```go
if c.UI.SidebarWidth == 0 {
    userTheme := c.UI.Theme
    c.UI = d.UI  // full replacement
    ...
}
```
A user who sets `show_pane_preview = false` or `event_log_lines = 50` but does not set `sidebar_width` loses all their UI settings on every config load. Fix: merge each field individually.

## Files

- Modify: `internal/hooks/install.go` — raise hook timeout constant
- Modify: `internal/hooks/handler.go` — CWD fallback for stale session ID; decouple sound from state error
- Modify: `internal/hooks/handler_test.go` — tests for EC-5 and EC-7
- Modify: `internal/config/config.go` — `Sound.Enabled` → `*bool`
- Modify: `internal/config/defaults.go` — `*bool` init + field-by-field UI merge
- Modify: `internal/config/config_test.go` — tests for EC-9 and EC-10

---

### Task 1: Raise hook timeout from 2s to 5s

**Files:**
- Modify: `internal/hooks/install.go`

- [ ] **Step 1: Write failing test**

Add to `internal/hooks/install_test.go` (check the file exists first with `ls`; add if needed):

```go
func TestClaudeHookTimeoutIs5Seconds(t *testing.T) {
	entries := ExpectedClaudeEntries("/usr/local/bin/cleo")
	for ev, rawEntry := range entries {
		entries, ok := rawEntry.([]any)
		if !ok || len(entries) == 0 {
			t.Fatalf("event %s: unexpected shape %T", ev, rawEntry)
		}
		entry, ok := entries[0].(map[string]any)
		if !ok {
			t.Fatalf("event %s: entry not a map", ev)
		}
		hooks, ok := entry["hooks"].([]any)
		if !ok || len(hooks) == 0 {
			t.Fatalf("event %s: no hooks", ev)
		}
		hook, ok := hooks[0].(map[string]any)
		if !ok {
			t.Fatalf("event %s: hook not a map", ev)
		}
		if timeout, _ := hook["timeout"].(int); timeout != 5 {
			t.Errorf("event %s: want timeout 5, got %v", ev, hook["timeout"])
		}
	}
}

func TestCodexHookTimeoutIs5Seconds(t *testing.T) {
	entries := ExpectedCodexEntries("/usr/local/bin/cleo")
	for ev, rawEntry := range entries {
		entries, ok := rawEntry.([]any)
		if !ok || len(entries) == 0 {
			t.Fatalf("event %s: unexpected shape %T", ev, rawEntry)
		}
		entry, ok := entries[0].(map[string]any)
		if !ok {
			t.Fatalf("event %s: entry not a map", ev)
		}
		hooks, ok := entry["hooks"].([]any)
		if !ok || len(hooks) == 0 {
			t.Fatalf("event %s: no hooks", ev)
		}
		hook, ok := hooks[0].(map[string]any)
		if !ok {
			t.Fatalf("event %s: hook not a map", ev)
		}
		if timeout, _ := hook["timeout"].(int); timeout != 5 {
			t.Errorf("event %s: want timeout 5, got %v", ev, hook["timeout"])
		}
	}
}
```

- [ ] **Step 2: Run tests to confirm they fail**

```bash
cd /Users/dhruvsaxena/Dev/dhruvsaxena1998/cleo
go test ./internal/hooks/ -run "TestClaudeHookTimeoutIs5Seconds|TestCodexHookTimeoutIs5Seconds" -v
```

Expected: FAIL — `want timeout 5, got 2`.

- [ ] **Step 3: Raise the timeout in both entry builders**

In `internal/hooks/install.go`, find the two occurrences of `"timeout": 2` (in `ExpectedClaudeEntries` around line 31 and `ExpectedCodexEntries` around line 85). Change both to `"timeout": 5`.

`ExpectedClaudeEntries` (around line 27–35):
```go
out[ev] = []any{
    map[string]any{
        "hooks": []any{
            map[string]any{
                "type":    "command",
                "command": fmt.Sprintf("%s hook claude %s", cleoBin, ev),
                "timeout": 5,
            },
        },
    },
}
```

`ExpectedCodexEntries` (around line 79–89):
```go
out[ev] = []any{
    map[string]any{
        "hooks": []any{
            map[string]any{
                "type":    "command",
                "command": fmt.Sprintf("%s hook codex %s", cleoBin, ev),
                "timeout": 5,
            },
        },
    },
}
```

- [ ] **Step 4: Run all hook tests**

```bash
go test ./internal/hooks/ -v
```

Expected: all PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/hooks/install.go internal/hooks/install_test.go
git commit -m "fix(hooks): raise Claude/Codex hook timeout from 2s to 5s to reduce lock-contention drops"
```

---

### Task 2: CWD fallback for stale `CLEO_SESSION_ID` (Claude)

**Files:**
- Modify: `internal/hooks/handler.go`
- Modify: `internal/hooks/handler_test.go`

- [ ] **Step 1: Write failing test**

Add to `internal/hooks/handler_test.go`:

```go
func TestClaudeStaleSidFallsBackToCwd(t *testing.T) {
	deps, st, _ := setup(t)
	_, _ = st.Apply("cleo-x-claude-1", state.EvSessionStart, "")

	// Simulate: env var is set, but points to a session not in the store.
	deps.Now = func() (string, error) { return "stale-session-id", nil }
	deps.FindByCwd = func(cwd, agent string) (string, error) {
		if cwd == "/tmp/myproject" && agent == "claude" {
			return "cleo-x-claude-1", nil
		}
		return "", nil
	}

	payload := []byte(`{"cwd":"/tmp/myproject","message":"Need approval"}`)
	if err := Handle(deps, "claude", "Notification", payload); err != nil {
		t.Fatal(err)
	}
	got, _ := st.Get("cleo-x-claude-1")
	if got.State != state.WaitingForInput {
		t.Errorf("expected WaitingForInput via stale-sid CWD fallback, got %s", got.State)
	}
}
```

- [ ] **Step 2: Run test to confirm it fails**

```bash
go test ./internal/hooks/ -run TestClaudeStaleSidFallsBackToCwd -v
```

Expected: FAIL — the notification is silently dropped because CWD fallback is not attempted for Claude.

- [ ] **Step 3: Extend the CWD fallback condition in `resolveSession`**

In `internal/hooks/handler.go`, the current condition at line 90:

```go
if (err != nil || sid == "") && proto.UsesCwdFallback() && d.FindByCwd != nil {
```

Replace with:

```go
staleSid := trace.FallbackReason == "env_unknown_session"
if (err != nil || sid == "") && d.FindByCwd != nil && (proto.UsesCwdFallback() || staleSid) {
```

The full updated `resolveSession` function for clarity — only this one line changes inside the if-condition:

```go
func resolveSession(d Deps, proto Protocol, event string, payload []byte) string {
	trace := hookTrace{Protocol: proto.Name(), Event: event, EnvSession: os.Getenv("CLEO_SESSION_ID") != ""}

	sid, err := d.Now()
	if err == nil {
		if d.State != nil {
			if _, sErr := d.State.Get(sid); sErr != nil {
				trace.FallbackReason = "env_unknown_session"
				err = sErr
				sid = ""
			} else {
				trace.FallbackReason = "env_present"
				trace.ResolvedSession = sid
			}
		} else {
			trace.FallbackReason = "env_present"
			trace.ResolvedSession = sid
		}
	} else {
		trace.FallbackReason = "env_missing"
	}

	staleSid := trace.FallbackReason == "env_unknown_session"
	if (err != nil || sid == "") && d.FindByCwd != nil && (proto.UsesCwdFallback() || staleSid) {
		var base struct {
			Cwd string `json:"cwd"`
		}
		_ = json.Unmarshal(payload, &base)
		trace.Cwd = base.Cwd
		if base.Cwd == "" {
			if wd, wdErr := os.Getwd(); wdErr == nil {
				base.Cwd = wd
				trace.Cwd = wd
			}
		}
		if base.Cwd != "" {
			resolved, fbErr := d.FindByCwd(base.Cwd, proto.Name())
			if fbErr != nil || resolved == "" {
				trace.FallbackReason = "no_match"
				err = fbErr
			} else {
				trace.ResolvedSession = resolved
				sid = resolved
				err = nil
			}
		}
	}

	if err != nil || sid == "" {
		trace.Result = "ignored:no_session"
		logHookTrace(d.Paths, trace)
		if trace.FallbackReason == "no_match" {
			logHookErr(d.Paths, proto.Name(), event, fmt.Errorf("no session matched cwd=%q", trace.Cwd))
		}
		return ""
	}
	trace.Result = "resolved"
	logHookTrace(d.Paths, trace)
	return sid
}
```

- [ ] **Step 4: Verify existing CWD-guard tests still pass**

The tests `TestClaudeStandaloneSessionIgnoredWhenNoEnvVar` and `TestResolveSession_CwdFallbackNotCalledForClaude` both use `d.Now = func() (string, error) { return "", fmt.Errorf("not set") }` — env var MISSING, not stale. `trace.FallbackReason` will be `"env_missing"`, not `"env_unknown_session"`, so `staleSid = false` and the guard still blocks CWD fallback for those cases.

```bash
go test ./internal/hooks/ -v
```

Expected: all PASS including the new test.

- [ ] **Step 5: Commit**

```bash
git add internal/hooks/handler.go internal/hooks/handler_test.go
git commit -m "fix(hooks): attempt CWD fallback for Claude when CLEO_SESSION_ID is stale/unknown"
```

---

### Task 3: Decouple sound from `state.Apply` failure

**Files:**
- Modify: `internal/hooks/handler.go`
- Modify: `internal/hooks/handler_test.go`

- [ ] **Step 1: Write failing test**

Add to `internal/hooks/handler_test.go`:

```go
type errorStateStore struct {
	inner *state.Store
}

func (e *errorStateStore) Apply(id string, ev state.Event, msg string) (state.Session, error) {
	return state.Session{}, fmt.Errorf("disk full")
}
func (e *errorStateStore) ApplySynthetic(id string, ev state.Event, msg string) (state.Session, error) {
	return state.Session{}, fmt.Errorf("disk full")
}
func (e *errorStateStore) Get(id string) (state.Session, error)     { return e.inner.Get(id) }
func (e *errorStateStore) Put(s state.Session) error                { return e.inner.Put(s) }
func (e *errorStateStore) List() ([]state.Session, error)           { return e.inner.List() }
func (e *errorStateStore) Delete(id string) error                   { return nil }

func TestSoundPlaysEvenWhenStateApplyFails(t *testing.T) {
	deps, st, _ := setup(t)
	player := &recordingPlayer{}
	deps.Sound = player
	_, _ = st.Apply("cleo-x-claude-1", state.EvSessionStart, "")

	// Wrap state store to make Apply fail.
	deps.State = &errorStateStore{inner: st}

	err := Handle(deps, "claude", "Notification", []byte(`{"message":"Need approval"}`))
	if err == nil {
		t.Error("expected error from failed state apply")
	}
	if len(player.played) != 1 {
		t.Errorf("expected sound to play despite state error, played %v", player.played)
	}
}
```

You also need a `StateApplier` interface in the `Deps` struct. See Step 3 for the implementation detail.

- [ ] **Step 2: Run test to confirm it fails**

```bash
go test ./internal/hooks/ -run TestSoundPlaysEvenWhenStateApplyFails -v
```

Expected: FAIL — currently returns early on Apply error before reaching the sound call.

- [ ] **Step 3: Update `applyNormalized` to continue on error**

The `Deps.State` field is currently `*state.Store`. To support the test's `errorStateStore`, introduce a minimal interface. Add to `internal/hooks/handler.go` (near the top, after imports):

```go
// StateStore is the subset of state.Store used by the hook handler.
type StateStore interface {
	Apply(id string, ev state.Event, msg string) (state.Session, error)
	Get(id string) (state.Session, error)
}
```

Change the `Deps` struct field from:
```go
State  *state.Store
```
to:
```go
State  StateStore
```

Update `applyNormalized` to continue on Apply error:

```go
func applyNormalized(d Deps, sid string, norm NormalizedEvent) error {
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
	if norm.SoundEvent != "" && d.Config.SoundEventEnabled(norm.SoundEvent) && !sessionFocused(d, sid) {
		playSound(d, norm.SoundEvent)
	}
	return applyErr
}
```

- [ ] **Step 4: Fix compilation — `resolveSession` calls `d.State.Get`**

`resolveSession` currently calls `d.State.Get(sid)` at line 74. The `StateStore` interface already includes `Get`, so this compiles fine.

Also update the call site in `internal/cli/hook.go` where `Deps` is constructed — `c.State` is `*state.Store`, which implements the new `StateStore` interface, so no change needed there.

- [ ] **Step 5: Run all hook tests**

```bash
go test ./internal/hooks/ -v
```

Expected: all PASS.

- [ ] **Step 6: Run full suite**

```bash
go test ./...
```

Expected: all PASS.

- [ ] **Step 7: Commit**

```bash
git add internal/hooks/handler.go internal/hooks/handler_test.go
git commit -m "fix(hooks): play notification sound even when state.Apply fails (decouple sound from disk write)"
```

---

### Task 4: Fix `Sound.Enabled` not merged from defaults

**Files:**
- Modify: `internal/config/config.go`
- Modify: `internal/config/defaults.go`
- Modify: `internal/config/config_test.go`

- [ ] **Step 1: Write failing test**

Add to `internal/config/config_test.go`:

```go
func TestSoundEnabledDefaultsToTrueWhenAbsent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")

	// Write a config that has a [sound] section but no enabled key.
	if err := os.WriteFile(path, []byte("[sound]\nvolume = 0.5\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	c, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if !c.SoundEventEnabled("needs_input") {
		t.Error("sound should default to enabled when 'enabled' key is absent from config")
	}
}

func TestSoundEnabledFalseWhenExplicitlySet(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")

	if err := os.WriteFile(path, []byte("[sound]\nenabled = false\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	c, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if c.SoundEventEnabled("needs_input") {
		t.Error("sound should be disabled when enabled = false is set explicitly")
	}
}
```

- [ ] **Step 2: Run tests to confirm first test fails**

```bash
cd /Users/dhruvsaxena/Dev/dhruvsaxena1998/cleo
go test ./internal/config/ -run "TestSoundEnabledDefaultsToTrueWhenAbsent|TestSoundEnabledFalseWhenExplicitlySet" -v
```

Expected: `TestSoundEnabledDefaultsToTrueWhenAbsent` FAIL — currently reads `enabled` as `false` (Go zero value) and never fills it in.

- [ ] **Step 3: Change `Sound.Enabled` to `*bool` in `config.go`**

In `internal/config/config.go`, change the `Sound` struct:

```go
type Sound struct {
	Enabled      *bool             `toml:"enabled"`
	Volume       float64           `toml:"volume"`
	Events       map[string]string `toml:"events"`
	EventEnabled map[string]bool   `toml:"event_enabled"`
}
```

Update `SoundEventEnabled`:

```go
func (c Config) SoundEventEnabled(event string) bool {
	if c.Sound.Enabled != nil && !*c.Sound.Enabled {
		return false
	}
	if c.Sound.EventEnabled == nil {
		return true
	}
	enabled, ok := c.Sound.EventEnabled[event]
	if !ok {
		return true
	}
	return enabled
}
```

- [ ] **Step 4: Update `Defaults_()` and `mergeDefaults` in `defaults.go`**

In `Defaults_()`, change the Sound initialization:

```go
Sound: Sound{
	Enabled: func() *bool { b := true; return &b }(),
	Volume:  0.7,
	Events: map[string]string{
		"session_start":     "start.wav",
		"needs_input":       "attention.wav",
		"session_idle":      "done.wav",
		"session_completed": "done.wav",
		"session_error":     "error.wav",
	},
	EventEnabled: map[string]bool{
		"session_start":     true,
		"needs_input":       true,
		"session_idle":      true,
		"session_completed": true,
		"session_error":     true,
	},
},
```

In `mergeDefaults`, add Sound.Enabled fill-in after the Sound.Volume check:

```go
if c.Sound.Enabled == nil {
	enabled := true
	c.Sound.Enabled = &enabled
}
if c.Sound.Volume == 0 {
	c.Sound.Volume = d.Sound.Volume
}
```

- [ ] **Step 5: Fix the existing `TestLoadDefaults` test**

The existing test checks `if !c.Sound.Enabled` (plain bool). Update to dereference:

```go
if c.Sound.Enabled == nil || !*c.Sound.Enabled {
    t.Errorf("sound default disabled")
}
```

- [ ] **Step 6: Fix `TestPartialSoundEventEnabledMergesDefaults` — it passes `Enabled: true` which is now a `*bool`**

Update the Save call in that test:

```go
enabled := true
if err := Save(path, Config{
    Sound: Sound{
        Enabled: &enabled,
        Volume:  0.5,
        ...
    },
}); err != nil {
```

- [ ] **Step 7: Run all config tests**

```bash
go test ./internal/config/ -v
```

Expected: all PASS.

- [ ] **Step 8: Run full suite**

```bash
go test ./...
```

Expected: all PASS.

- [ ] **Step 9: Commit**

```bash
git add internal/config/config.go internal/config/defaults.go internal/config/config_test.go
git commit -m "fix(config): Sound.Enabled becomes *bool so absent key defaults to true instead of false"
```

---

### Task 5: Fix UI config wholesale replacement in `mergeDefaults`

**Files:**
- Modify: `internal/config/defaults.go`
- Modify: `internal/config/config_test.go`

- [ ] **Step 1: Write failing test**

Add to `internal/config/config_test.go`:

```go
func TestUISettingsPreservedWhenSidebarWidthAbsent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")

	// Write config with some UI settings but no sidebar_width.
	content := "[ui]\nshow_pane_preview = false\nevent_log_lines = 50\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	c, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if c.UI.ShowPanePreview {
		t.Error("show_pane_preview = false should be preserved, not overwritten by defaults")
	}
	if c.UI.EventLogLines != 50 {
		t.Errorf("event_log_lines = 50 should be preserved, got %d", c.UI.EventLogLines)
	}
	// SidebarWidth should be filled from defaults (it was absent).
	if c.UI.SidebarWidth == 0 {
		t.Error("sidebar_width should be filled from defaults when absent")
	}
}
```

- [ ] **Step 2: Run test to confirm it fails**

```bash
go test ./internal/config/ -run TestUISettingsPreservedWhenSidebarWidthAbsent -v
```

Expected: FAIL — `show_pane_preview` and `event_log_lines` are overwritten by the current full-struct replacement.

- [ ] **Step 3: Replace full struct replacement with field-by-field merge in `mergeDefaults`**

In `internal/config/defaults.go`, replace the entire UI merge block:

Current (around lines 76–85):
```go
if c.UI.SidebarWidth == 0 {
    userTheme := c.UI.Theme
    c.UI = d.UI
    if userTheme != "" {
        c.UI.Theme = userTheme
    }
}
if c.UI.Theme == "" {
    c.UI.Theme = d.UI.Theme
}
```

Replace with:
```go
if c.UI.SidebarWidth == 0 {
    c.UI.SidebarWidth = d.UI.SidebarWidth
}
if c.UI.PanePreviewLines == 0 {
    c.UI.PanePreviewLines = d.UI.PanePreviewLines
}
if c.UI.PanePreviewInterval == 0 {
    c.UI.PanePreviewInterval = d.UI.PanePreviewInterval
}
if c.UI.EventLogLines == 0 {
    c.UI.EventLogLines = d.UI.EventLogLines
}
if c.UI.Theme == "" {
    c.UI.Theme = d.UI.Theme
}
```

> Note: `ShowPanePreview` is a `bool` whose default is `true` but whose TOML zero value is `false`. We cannot distinguish "user set false" from "not set" without changing to `*bool`. This is a known limitation — treat it as a future refactor. Users who want to disable pane preview explicitly (`show_pane_preview = false`) should also set at least one other UI field (e.g., `sidebar_width`) to prevent their setting from being indistinguishable from "no UI section".

- [ ] **Step 4: Run all config tests**

```bash
go test ./internal/config/ -v
```

Expected: all PASS.

- [ ] **Step 5: Run full suite**

```bash
go test ./...
```

Expected: all PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/config/defaults.go internal/config/config_test.go
git commit -m "fix(config): merge UI fields individually instead of replacing entire UI block when sidebar_width is absent"
```
