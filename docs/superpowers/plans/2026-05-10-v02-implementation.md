# cleo v0.2 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Ship the v0.2 design (`docs/superpowers/specs/2026-05-10-v02-design.md`) — six functional changes plus three hygiene ride-alongs that together make cleo trustworthy for daily use.

**Architecture:** Each task below is one PR. Tasks land in the order listed (CI first, then docs scaffold, then code in dependency order). Within each task, every code change starts with a failing test. Hygiene-only tasks (1, 2, 9) skip TDD because they have no behavior to assert.

**Tech Stack:** Go 1.25.5, Bubble Tea + Lipgloss + Cobra (existing). No new dependencies.

---

## Working assumptions

- Run all commands from the repo root: `/Users/dhruvsaxena/Dev/dhruvsaxena1998/cleo` (replace with your checkout root).
- Every commit message uses Conventional Commits (`feat:`, `fix:`, `chore:`, `docs:`, `refactor:`, `ci:`, `test:`).
- Every code-changing PR also adds a one-line entry under `## [Unreleased]` in `CHANGELOG.md`. Commit the changelog edit alongside the code change in the same commit unless noted otherwise.
- Verification command after every commit: `go build -o /tmp/cleo-build ./cmd/cleo && go vet ./... && go test ./... -count=1`. The plan does not repeat this between every step; run it at the end of each task before opening a PR.

---

## Task 1: CI test workflow on PRs (spec §4.1)

**Goal:** Every PR and every push to `main` runs `go vet` + `go test ./...` automatically.

**Files:**
- Create: `.github/workflows/test.yml`

- [ ] **Step 1.1: Create the workflow file**

```yaml
# .github/workflows/test.yml
name: test

on:
  push:
    branches: [main]
  pull_request:
    branches: [main]

permissions:
  contents: read

jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: '1.25.5'
          cache: true
      - run: go vet ./...
      - run: go test ./... -count=1
```

- [ ] **Step 1.2: Validate the YAML locally**

Run: `python3 -c "import yaml; yaml.safe_load(open('.github/workflows/test.yml'))"`
Expected: no output (parse success).

- [ ] **Step 1.3: Commit**

```bash
git add .github/workflows/test.yml
git commit -m "ci: run go vet and go test on PRs and pushes to main"
```

- [ ] **Step 1.4: Push branch and open PR**

The PR triggers the workflow; verify it succeeds before merging. After merge, every later task's PR is gated by this CI.

---

## Task 2: CHANGELOG.md scaffold (spec §4.2)

**Goal:** Establish a Keep-A-Changelog file with the v0.1.0-alpha.1 retrospective entry. Subsequent tasks add entries under `[Unreleased]` as they ship.

**Files:**
- Create: `CHANGELOG.md`

- [ ] **Step 2.1: Create the file**

```markdown
# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [0.1.0-alpha.1] - 2026-05-09

First public alpha. Terminal session manager for AI coding agents.

### Added
- TUI dashboard with project sidebar, event log, and pane mirror
- Hook-based lifecycle tracking for Claude Code and Codex
- 5 built-in themes with terminal background sync (OSC 11/111)
- CLI: `add`, `rm`, `ls` (`--json`, age column), `run`, `attach`, `kill`, `prune`, `rename`, `init`, `cleanup`, `doctor`, `focus`
- Cross-platform sound playback (macOS afplay, Linux paplay/aplay/play) with focus-aware suppression
- Persistent state at `~/.config/cleo` with archived event logs
- Goreleaser-based release pipeline with homebrew tap

### Known limitations
- opencode and pi are managed-only (no hooks plugin)
- No Windows support
```

- [ ] **Step 2.2: Commit**

```bash
git add CHANGELOG.md
git commit -m "docs: add CHANGELOG.md with v0.1.0-alpha.1 retrospective entry"
```

---

## Task 3: Reconciler synthetic-event clock-reset fix + transition tests (spec §3.1)

**Goal:** Fix the bug where sessions ending in `WaitingForInput` never reach `Completed` because reconciler-driven `EvIdleTimeout` bumps `LastEventAt` and restarts the idle timer.

**Files:**
- Modify: `internal/state/store.go`
- Modify: `internal/state/state_test.go`
- Modify: `internal/reconcile/reconcile.go`
- Modify: `internal/reconcile/reconcile_test.go`
- Modify: `internal/cli/ls.go`

### Sub-task 3A: Add `ApplySynthetic` to state.Store

A synthetic event updates `State` (and optionally `LastMessage`) without bumping `LastEventAt`.

- [ ] **Step 3A.1: Write the failing test**

Append to `internal/state/state_test.go`:

```go
func TestApplySyntheticDoesNotBumpLastEventAt(t *testing.T) {
	dir := t.TempDir()
	st := NewStore(filepath.Join(dir, "state.json"), filepath.Join(dir, "state.json.lock"))
	at := time.Date(2026, 5, 1, 12, 0, 0, 0, time.UTC)
	if err := st.Put(Session{ID: "s1", State: WaitingForInput, LastEventAt: at}); err != nil {
		t.Fatalf("put: %v", err)
	}

	out, err := st.ApplySynthetic("s1", EvIdleTimeout, "")
	if err != nil {
		t.Fatalf("apply synthetic: %v", err)
	}

	if out.State != Idle {
		t.Errorf("state: want Idle, got %s", out.State)
	}
	if !out.LastEventAt.Equal(at) {
		t.Errorf("LastEventAt was bumped: want %v, got %v", at, out.LastEventAt)
	}
}
```

- [ ] **Step 3A.2: Run test, expect failure**

Run: `go test ./internal/state/ -run TestApplySynthetic -v`
Expected: compile error — `st.ApplySynthetic undefined`.

- [ ] **Step 3A.3: Implement `ApplySynthetic`**

In `internal/state/store.go`, add this method after `Apply`:

```go
// ApplySynthetic transitions a session by a reconciler-driven event without
// bumping LastEventAt. Use this for synthetic events (EvIdleTimeout, EvDead)
// that represent the absence of activity rather than activity itself.
// Bumping LastEventAt for these would reset idle timers and prevent stuck
// sessions from progressing.
func (s *Store) ApplySynthetic(id string, ev Event, lastMessage string) (Session, error) {
	var out Session
	err := s.modify(func(f *fileFormat) error {
		sess, ok := f.Sessions[id]
		if !ok {
			return ErrSessionNotFound
		}
		sess.State = NextState(sess.State, ev)
		if lastMessage != "" {
			sess.LastMessage = lastMessage
		}
		f.Sessions[id] = sess
		out = sess
		return nil
	})
	return out, err
}
```

- [ ] **Step 3A.4: Run test, expect pass**

Run: `go test ./internal/state/ -run TestApplySynthetic -v`
Expected: PASS.

- [ ] **Step 3A.5: Commit**

```bash
git add internal/state/store.go internal/state/state_test.go
git commit -m "feat(state): add ApplySynthetic that does not bump LastEventAt"
```

### Sub-task 3B: Reconciler uses `ApplySynthetic` for `EvIdleTimeout` and `EvDead`

- [ ] **Step 3B.1: Write the failing test**

Append to `internal/reconcile/reconcile_test.go`. (If the file imports state, time, testing, paths, etc., reuse them; otherwise add as needed.)

```go
func TestWaitingForInputProgressesToCompletedAcrossTwoIdleCycles(t *testing.T) {
	dir := t.TempDir()
	st := state.NewStore(filepath.Join(dir, "state.json"), filepath.Join(dir, "state.json.lock"))
	tx := &fakeTmuxLs{names: []string{"s1"}}

	tenMinAgo := time.Now().Add(-10 * time.Minute)
	if err := st.Put(state.Session{ID: "s1", State: state.WaitingForInput, LastEventAt: tenMinAgo}); err != nil {
		t.Fatalf("put: %v", err)
	}

	// First reconcile: WaitingForInput -> Idle. LastEventAt must NOT be bumped.
	if err := RunOpts(st, tx, Options{IdleTimeout: time.Minute, SpawningTimeout: 30 * time.Second}); err != nil {
		t.Fatalf("first reconcile: %v", err)
	}
	got, _ := st.Get("s1")
	if got.State != state.Idle {
		t.Fatalf("after first reconcile, want Idle, got %s", got.State)
	}
	if !got.LastEventAt.Equal(tenMinAgo) {
		t.Fatalf("LastEventAt bumped: want %v, got %v", tenMinAgo, got.LastEventAt)
	}

	// Second reconcile (immediate): Idle -> Completed because LastEventAt is still 10min ago.
	if err := RunOpts(st, tx, Options{IdleTimeout: time.Minute, SpawningTimeout: 30 * time.Second}); err != nil {
		t.Fatalf("second reconcile: %v", err)
	}
	got, _ = st.Get("s1")
	if got.State != state.Completed {
		t.Fatalf("after second reconcile, want Completed, got %s", got.State)
	}
}

type fakeTmuxLs struct{ names []string }

func (f *fakeTmuxLs) LsPrefix(_ string) ([]string, error) { return f.names, nil }
```

If `fakeTmuxLs` already exists in the test file, reuse it. Confirm by reading the file before pasting; if there's a name conflict, rename the fake locally.

- [ ] **Step 3B.2: Run test, expect failure**

Run: `go test ./internal/reconcile/ -run TestWaitingForInputProgressesToCompleted -v`
Expected: FAIL — second reconcile leaves the session in `Idle` (or oscillates) because `Apply(EvIdleTimeout)` bumps `LastEventAt`.

- [ ] **Step 3B.3: Switch reconciler to `ApplySynthetic`**

Edit `internal/reconcile/reconcile.go`. Replace `_, _ = st.Apply(s.ID, state.EvDead, "")` with `_, _ = st.ApplySynthetic(s.ID, state.EvDead, "")`. Replace `_, _ = st.Apply(s.ID, state.EvIdleTimeout, "")` with `_, _ = st.ApplySynthetic(s.ID, state.EvIdleTimeout, "")`. The `EvSessionStart` advance from `Spawning` stays as `Apply` (it represents real progression and `LastEventAt` should advance).

- [ ] **Step 3B.4: Run test, expect pass**

Run: `go test ./internal/reconcile/ -run TestWaitingForInputProgressesToCompleted -v`
Expected: PASS.

- [ ] **Step 3B.5: Run full reconcile package**

Run: `go test ./internal/reconcile/ -count=1 -v`
Expected: all PASS. Existing tests should still hold; if any test asserted that synthetic events bump `LastEventAt`, that assertion was wrong — adjust the test to assert `LastEventAt` is unchanged.

- [ ] **Step 3B.6: Commit**

```bash
git add internal/reconcile/reconcile.go internal/reconcile/reconcile_test.go
git commit -m "fix(reconcile): use ApplySynthetic so idle-timeout doesn't reset clock

Sessions in WaitingForInput could never reach Completed because the
reconciler-driven EvIdleTimeout transition bumped LastEventAt, restarting
the idle window. Reconciler now uses ApplySynthetic for EvDead and
EvIdleTimeout, keeping the LastEventAt anchor stable across timeout
cycles."
```

### Sub-task 3C: Reconciler annotates the spawning-timeout advance

When `Spawning → Running` fires because `SpawningTimeout` elapsed without a startup hook, set `LastMessage` so the user can see what happened in the events panel.

- [ ] **Step 3C.1: Write the failing test**

Append to `internal/reconcile/reconcile_test.go`:

```go
func TestSpawningTimeoutAdvanceSetsLastMessage(t *testing.T) {
	dir := t.TempDir()
	st := state.NewStore(filepath.Join(dir, "state.json"), filepath.Join(dir, "state.json.lock"))
	tx := &fakeTmuxLs{names: []string{"s1"}}

	if err := st.Put(state.Session{
		ID: "s1", State: state.Spawning,
		StartedAt: time.Now().Add(-time.Minute),
	}); err != nil {
		t.Fatalf("put: %v", err)
	}

	if err := RunOpts(st, tx, Options{IdleTimeout: 10 * time.Minute, SpawningTimeout: 5 * time.Second}); err != nil {
		t.Fatalf("reconcile: %v", err)
	}
	got, _ := st.Get("s1")
	if got.State != state.Running {
		t.Fatalf("want Running, got %s", got.State)
	}
	if !strings.Contains(got.LastMessage, "spawning") {
		t.Fatalf("LastMessage should mention spawning, got %q", got.LastMessage)
	}
}
```

- [ ] **Step 3C.2: Run test, expect failure**

Run: `go test ./internal/reconcile/ -run TestSpawningTimeoutAdvanceSetsLastMessage -v`
Expected: FAIL — `LastMessage` is empty.

- [ ] **Step 3C.3: Implement**

In `internal/reconcile/reconcile.go`, change the spawning-timeout branch:

```go
if s.State == state.Spawning && liveSet[s.ID] &&
    opts.SpawningTimeout > 0 && time.Since(s.StartedAt) > opts.SpawningTimeout {
    _, _ = st.Apply(s.ID, state.EvSessionStart,
        "advanced from spawning by reconciler (no startup hook seen)")
    continue
}
```

- [ ] **Step 3C.4: Run test, expect pass**

Run: `go test ./internal/reconcile/ -run TestSpawningTimeoutAdvanceSetsLastMessage -v`
Expected: PASS.

- [ ] **Step 3C.5: Commit**

```bash
git add internal/reconcile/reconcile.go internal/reconcile/reconcile_test.go
git commit -m "feat(reconcile): annotate spawning-timeout advance with LastMessage"
```

### Sub-task 3D: Drop `reconcile.Run`, unify on `RunOpts`

`internal/cli/ls.go:56` calls the legacy `reconcile.Run(...)` which hardcodes `SpawningTimeout = 30s` and ignores the configured value.

- [ ] **Step 3D.1: Write the failing test**

Append to `internal/reconcile/reconcile_test.go`:

```go
func TestRunOptsUsesProvidedSpawningTimeout(t *testing.T) {
	dir := t.TempDir()
	st := state.NewStore(filepath.Join(dir, "state.json"), filepath.Join(dir, "state.json.lock"))
	tx := &fakeTmuxLs{names: []string{"s1"}}

	// Started 5s ago; SpawningTimeout = 1s should fire, default 30s should not.
	if err := st.Put(state.Session{
		ID: "s1", State: state.Spawning,
		StartedAt: time.Now().Add(-5 * time.Second),
	}); err != nil {
		t.Fatalf("put: %v", err)
	}

	if err := RunOpts(st, tx, Options{SpawningTimeout: time.Second, IdleTimeout: time.Hour}); err != nil {
		t.Fatalf("reconcile: %v", err)
	}
	got, _ := st.Get("s1")
	if got.State != state.Running {
		t.Fatalf("with 1s timeout and 5s elapsed, want Running, got %s", got.State)
	}
}
```

- [ ] **Step 3D.2: Confirm it passes already**

Run: `go test ./internal/reconcile/ -run TestRunOptsUsesProvidedSpawningTimeout -v`
Expected: PASS already (this just locks in current `RunOpts` behavior). Move on if so.

- [ ] **Step 3D.3: Drop `reconcile.Run`**

In `internal/reconcile/reconcile.go`, delete the `Run` function (lines that read `func Run(st ..., idleTimeout time.Duration) error { return RunOpts(...) }`). Keep only `RunOpts`.

- [ ] **Step 3D.4: Update the only caller**

In `internal/cli/ls.go`, change line 56:

Before:
```go
_ = reconcile.Run(c.State, c.Tmux, c.Config.Retention.IdleToCompletedTimeout)
```

After:
```go
_ = reconcile.RunOpts(c.State, c.Tmux, reconcile.Options{
    IdleTimeout:     c.Config.Retention.IdleToCompletedTimeout,
    SpawningTimeout: c.Config.Retention.SpawningTimeout,
})
```

- [ ] **Step 3D.5: Run all tests**

Run: `go vet ./... && go test ./... -count=1`
Expected: PASS. The `cli` package may have a test that called `reconcile.Run`; if so, update it to call `reconcile.RunOpts(...)` with explicit options (or just rely on the `cleo ls` higher-level test path).

- [ ] **Step 3D.6: Commit**

```bash
git add internal/reconcile/reconcile.go internal/reconcile/reconcile_test.go internal/cli/ls.go
git commit -m "refactor(reconcile): drop legacy Run; cleo ls uses configured SpawningTimeout"
```

### Sub-task 3E: Update CHANGELOG and finish

- [ ] **Step 3E.1: Add entry under `[Unreleased]`**

Edit `CHANGELOG.md`. Under `## [Unreleased]`, append:

```markdown
### Fixed
- Sessions in `waiting_for_input` now correctly progress to `completed` after two idle-timeout windows. Previously, the synthetic `idle_timeout` event bumped `last_event_at`, restarting the timer indefinitely.

### Changed
- Reconciler now annotates a spawning-timeout advance with `LastMessage = "advanced from spawning by reconciler (no startup hook seen)"`, surfaced in the TUI events panel.
- `cleo ls` honors `retention.spawning_timeout` (previously hardcoded to 30s).
```

- [ ] **Step 3E.2: Final task verification**

Run: `go vet ./... && go test ./... -count=1`
Expected: PASS.

- [ ] **Step 3E.3: Commit changelog edit**

```bash
git add CHANGELOG.md
git commit -m "docs(changelog): record reconciler synthetic-event fix"
```

---

## Task 4: Hook attribution traces gain `fallback_reason` (spec §1.3)

**Goal:** Each entry in `hook-trace.log` records why session resolution took the path it did. Pure logging change — no behavior change. Consumed by Task 6 (doctor upgrade).

**Files:**
- Modify: `internal/hooks/handler.go`
- Modify: `internal/hooks/handler_test.go`
- Modify: `internal/cli/doctor.go` (only the `hookTraceRow` struct, to consume the new field)

### Sub-task 4A: Extend `hookTrace` and emit `fallback_reason`

- [ ] **Step 4A.1: Write the failing test**

Append to `internal/hooks/handler_test.go`. The test reads the on-disk trace log after invoking `Handle` and asserts the JSON has the expected `fallback_reason`.

```go
func TestFallbackReasonEnvPresent(t *testing.T) {
	d, paths := newTestDeps(t)
	d.Now = func() (string, error) { return "sid-1", nil }
	if err := Handle(d, "claude", "PreToolUse", strings.NewReader(`{}`), io.Discard); err != nil {
		t.Fatalf("handle: %v", err)
	}
	row := lastTraceRow(t, paths.HookTraceLog())
	if row.FallbackReason != "env_present" {
		t.Errorf("fallback_reason: want env_present, got %q", row.FallbackReason)
	}
}

func TestFallbackReasonEnvMissing(t *testing.T) {
	d, paths := newTestDeps(t)
	// Now returns errNoSession; FindByCwd is not configured (claude path)
	d.Now = func() (string, error) { return "", errNoSessionTest }
	_ = Handle(d, "claude", "PreToolUse", strings.NewReader(`{}`), io.Discard)
	row := lastTraceRow(t, paths.HookTraceLog())
	if row.FallbackReason != "env_missing" {
		t.Errorf("fallback_reason: want env_missing, got %q", row.FallbackReason)
	}
}

func TestFallbackReasonEnvUnknownSession(t *testing.T) {
	d, paths := newTestDeps(t)
	d.Now = func() (string, error) { return "stale-sid", nil }
	// Override Apply path: simulate stale-sid not in state by making Now return a sid
	// that the handler will try to apply against an empty store; it'll fail. The trace
	// row should still be written with fallback_reason = env_unknown_session.
	// (Implementation detail: handler sets fallback_reason = env_unknown_session when
	// the resolved session does not exist in state.)
	_ = Handle(d, "claude", "PreToolUse", strings.NewReader(`{}`), io.Discard)
	row := lastTraceRow(t, paths.HookTraceLog())
	if row.FallbackReason != "env_unknown_session" {
		t.Errorf("fallback_reason: want env_unknown_session, got %q", row.FallbackReason)
	}
}

func TestFallbackReasonNoMatchCodex(t *testing.T) {
	d, paths := newTestDeps(t)
	d.Now = func() (string, error) { return "", errNoSessionTest }
	d.FindByCwd = func(cwd, agent string) (string, error) {
		return "", os.ErrNotExist
	}
	_ = Handle(d, "codex", "PreToolUse", strings.NewReader(`{"cwd":"/some/path"}`), io.Discard)
	row := lastTraceRow(t, paths.HookTraceLog())
	if row.FallbackReason != "no_match" {
		t.Errorf("fallback_reason: want no_match, got %q", row.FallbackReason)
	}
}

func TestFallbackReasonMultiMatchFirstCodex(t *testing.T) {
	d, paths := newTestDeps(t)
	d.Now = func() (string, error) { return "", errNoSessionTest }
	d.FindByCwd = func(cwd, agent string) (string, error) {
		// Indicate via a sentinel sid suffix that there were multiple candidates.
		// Production FindByCwd returns the most-recent match plus a "multi" hint;
		// for now the test asserts the handler's plumbing, see implementation.
		return "match-sid", nil
	}
	// We'll configure FindByCwd to also signal multi via a wrapper interface
	// in the implementation step. For this test, the handler should detect multi
	// using a context hint set by a dedicated helper. See impl in 4A.3.
	_ = Handle(d, "codex", "PreToolUse", strings.NewReader(`{"cwd":"/some/path"}`), io.Discard)
	row := lastTraceRow(t, paths.HookTraceLog())
	if row.FallbackReason != "multi_match_first" && row.FallbackReason != "env_present" {
		// Allow env_present too if the test seam below isn't wired; the impl
		// step will tighten this. Worst case: open a follow-up to refine.
		t.Errorf("fallback_reason: want multi_match_first, got %q", row.FallbackReason)
	}
}

// lastTraceRow reads the trace log and returns the last decoded row.
// hookTraceRow is the same struct cli/doctor.go uses; redeclare locally for the test.
type traceRowForTest struct {
	At              string `json:"at"`
	Protocol        string `json:"protocol"`
	Event           string `json:"event"`
	Cwd             string `json:"cwd"`
	EnvSession      bool   `json:"env_session"`
	ResolvedSession string `json:"resolved_session"`
	Result          string `json:"result"`
	FallbackReason  string `json:"fallback_reason"`
}

func lastTraceRow(t *testing.T, path string) traceRowForTest {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read trace: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) == 0 {
		t.Fatalf("no trace rows at %s", path)
	}
	var row traceRowForTest
	if err := json.Unmarshal([]byte(lines[len(lines)-1]), &row); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	return row
}

var errNoSessionTest = errors.New("no session")
```

If `newTestDeps` doesn't already exist in `handler_test.go`, it should be a small helper that returns a `Deps` with a tempdir-backed `state.Store`, in-memory event log, and a `paths.Paths` rooted at the tempdir. Reuse whatever helper the existing tests already use (read the file first); if no helper exists, create one at the bottom of the file like:

```go
func newTestDeps(t *testing.T) (Deps, paths.Paths) {
	t.Helper()
	dir := t.TempDir()
	p := paths.NewWithRoot(dir)
	st := state.NewStore(p.StateFile(), p.StateLock())
	if err := st.Put(state.Session{ID: "sid-1", State: state.Running, ProjectID: "p", Agent: "claude"}); err != nil {
		t.Fatalf("seed: %v", err)
	}
	d := Deps{
		Paths:   p,
		State:   st,
		Config:  config.Defaults(),
		Events:  func(sid string) *events.Log { return events.NewLog(p.EventsLog(sid)) },
		Sound:   noopPlayer{},
		Focused: func(string) bool { return false },
	}
	return d, p
}

type noopPlayer struct{}

func (noopPlayer) Play(string) error { return nil }
func (noopPlayer) Available() bool   { return false }
```

- [ ] **Step 4A.2: Run test, expect failure**

Run: `go test ./internal/hooks/ -run TestFallbackReason -v`
Expected: FAIL — `FallbackReason` field is missing from the trace row JSON (the JSON value will be empty string).

- [ ] **Step 4A.3: Implement**

In `internal/hooks/handler.go`, modify the `hookTrace` struct and `Handle`:

```go
type hookTrace struct {
	Protocol        string `json:"protocol"`
	Event           string `json:"event"`
	EnvSession      bool   `json:"env_session"`
	Cwd             string `json:"cwd,omitempty"`
	ResolvedSession string `json:"resolved_session,omitempty"`
	Result          string `json:"result"`
	FallbackReason  string `json:"fallback_reason,omitempty"`
}
```

Replace the resolution block in `Handle` with:

```go
trace := hookTrace{Protocol: protocol, Event: event, EnvSession: os.Getenv("CLEO_SESSION_ID") != ""}
sid, err := d.Now()
if err == nil {
	trace.ResolvedSession = sid
	// Verify the resolved session actually exists in state. If not, fall through
	// to the cwd lookup (codex) or report env_unknown_session (claude).
	if d.State != nil {
		if _, sErr := d.State.Get(sid); sErr != nil {
			trace.FallbackReason = "env_unknown_session"
			err = sErr
			sid = ""
		} else {
			trace.FallbackReason = "env_present"
		}
	} else {
		trace.FallbackReason = "env_present"
	}
} else {
	trace.FallbackReason = "env_missing"
}

if (err != nil || sid == "") && d.FindByCwd != nil && protocol == "codex" {
	var base baseHookPayload
	_ = json.Unmarshal(body, &base)
	trace.Cwd = base.Cwd
	if base.Cwd == "" {
		if wd, wdErr := os.Getwd(); wdErr == nil {
			base.Cwd = wd
			trace.Cwd = wd
		}
	}
	if base.Cwd != "" {
		resolved, fbErr := d.FindByCwd(base.Cwd, protocol)
		if fbErr != nil || resolved == "" {
			trace.FallbackReason = "no_match"
			err = fbErr
		} else {
			trace.ResolvedSession = resolved
			sid = resolved
			err = nil
			// FindByCwd doesn't currently signal multi-match. Leave the existing
			// reason (env_missing or env_unknown_session) on the trace; the
			// resolved_session itself is the answer. multi_match_first will be
			// populated only once FindByCwd is enhanced — tracked in backlog.
		}
	}
}

if err != nil || sid == "" {
	trace.Result = "ignored:no_session"
	logHookTrace(d.Paths, trace)
	if trace.FallbackReason == "no_match" {
		logHookErr(d.Paths, protocol, event, fmt.Errorf("no session matched cwd=%q", trace.Cwd))
	}
	return nil
}
trace.Result = "resolved"
logHookTrace(d.Paths, trace)
```

Note the `multi_match_first` test in 4A.1 was deliberately permissive about its assertion. The full multi-match detection requires teaching `FindByCwd` to return additional context (e.g. a `MultiMatch bool`), which is beyond v0.2 scope — capture in backlog rather than implementing now. Adjust the test to either skip the `multi_match_first` assertion or remove that test entirely; its placeholder is documented.

Remove `TestFallbackReasonMultiMatchFirstCodex` from the test file — its premise can't be met without a `FindByCwd` interface change. Add an entry to `docs/superpowers/backlog.md`:

```markdown
### `FindByCwd` returns `multi_match_first` reason

- Today the cwd lookup returns one session ID; the handler can't tell if there were multiple candidates.
- Extend `FindByCwd` to return `(sid string, multi bool, err error)` and have the handler set `fallback_reason = "multi_match_first"` when `multi` is true.
- Small change but invasive (interface + all call sites + tests). Defer to v0.3.
```

- [ ] **Step 4A.4: Run tests, expect pass**

Run: `go test ./internal/hooks/ -count=1 -v`
Expected: PASS for the four remaining `TestFallbackReason*` cases. Existing tests should still pass; if any test reads the trace log JSON and asserts on full equality, update it to ignore the new optional `fallback_reason` field.

- [ ] **Step 4A.5: Update doctor's trace row consumer**

In `internal/cli/doctor.go`, extend `hookTraceRow`:

```go
type hookTraceRow struct {
	At              string `json:"at"`
	Protocol        string `json:"protocol"`
	Event           string `json:"event"`
	Cwd             string `json:"cwd"`
	EnvSession      bool   `json:"env_session"`
	ResolvedSession string `json:"resolved_session"`
	Result          string `json:"result"`
	FallbackReason  string `json:"fallback_reason"`
}
```

Run: `go vet ./... && go test ./... -count=1`
Expected: PASS.

- [ ] **Step 4A.6: Update CHANGELOG**

Edit `CHANGELOG.md` under `[Unreleased]` `### Added`:

```markdown
- Hook trace log entries now include a `fallback_reason` field documenting how the session was resolved (`env_present`, `env_missing`, `env_unknown_session`, `no_match`). Surfaced by `cleo doctor` in v0.2.
```

- [ ] **Step 4A.7: Commit**

```bash
git add internal/hooks/handler.go internal/hooks/handler_test.go internal/cli/doctor.go docs/superpowers/backlog.md CHANGELOG.md
git commit -m "feat(hooks): record fallback_reason on each trace log entry

env_present | env_missing | env_unknown_session | no_match. multi_match_first
deferred to v0.3 because it requires a FindByCwd interface change (see
docs/superpowers/backlog.md)."
```

---

## Task 5: `cleo events <session-id>` command (spec §1.1)

**Goal:** New top-level command to print and tail per-session JSONL event logs with type/since/limit filtering and JSON passthrough. Includes archive (gzip) lookup.

**Files:**
- Create: `internal/cli/events.go`
- Create: `internal/cli/events_test.go`
- Modify: `internal/cli/root.go` (register the command)
- Modify: `internal/events/log.go` (helper for filtered tailing)

### Sub-task 5A: Filtered read helper in `internal/events/log.go`

Add a helper that reads all entries and applies optional type/since/limit filters. Keep `Tail` for backward compat.

- [ ] **Step 5A.1: Write the failing test**

Append to `internal/events/events_test.go`:

```go
func TestReadFiltered(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "x.jsonl")
	log := NewLog(path)
	now := time.Now().UTC()
	older := now.Add(-2 * time.Hour)
	for _, e := range []Entry{
		{At: older, Type: "session_start"},
		{At: older.Add(time.Hour), Type: "notification"},
		{At: now, Type: "post_tool_use"},
	} {
		if err := log.Append(e); err != nil {
			t.Fatalf("append: %v", err)
		}
	}

	// No filters
	got, err := log.ReadFiltered(ReadOpts{})
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if len(got) != 3 {
		t.Errorf("len no-filter: want 3, got %d", len(got))
	}

	// Type filter
	got, _ = log.ReadFiltered(ReadOpts{Type: "notification"})
	if len(got) != 1 || got[0].Type != "notification" {
		t.Errorf("type filter: %+v", got)
	}

	// Since filter (last hour)
	got, _ = log.ReadFiltered(ReadOpts{Since: now.Add(-90 * time.Minute)})
	if len(got) != 1 || got[0].Type != "post_tool_use" {
		t.Errorf("since filter: %+v", got)
	}

	// Limit
	got, _ = log.ReadFiltered(ReadOpts{Limit: 2})
	if len(got) != 2 {
		t.Errorf("limit: want 2, got %d", len(got))
	}
	// Limit returns most recent
	if got[0].Type != "notification" || got[1].Type != "post_tool_use" {
		t.Errorf("limit ordering: %+v", got)
	}
}
```

- [ ] **Step 5A.2: Run test, expect failure**

Run: `go test ./internal/events/ -run TestReadFiltered -v`
Expected: FAIL — `ReadFiltered`/`ReadOpts` undefined.

- [ ] **Step 5A.3: Implement**

In `internal/events/log.go`, add at the bottom:

```go
type ReadOpts struct {
	Type  string
	Since time.Time
	Limit int
}

func (l *Log) ReadFiltered(opts ReadOpts) ([]Entry, error) {
	f, err := os.Open(l.path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var out []Entry
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 1<<20), 1<<20)
	for sc.Scan() {
		var e Entry
		if err := json.Unmarshal(sc.Bytes(), &e); err != nil {
			continue
		}
		if opts.Type != "" && e.Type != opts.Type {
			continue
		}
		if !opts.Since.IsZero() && e.At.Before(opts.Since) {
			continue
		}
		out = append(out, e)
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	if opts.Limit > 0 && len(out) > opts.Limit {
		out = out[len(out)-opts.Limit:]
	}
	return out, nil
}
```

- [ ] **Step 5A.4: Run test, expect pass**

Run: `go test ./internal/events/ -run TestReadFiltered -v`
Expected: PASS.

- [ ] **Step 5A.5: Commit**

```bash
git add internal/events/log.go internal/events/events_test.go
git commit -m "feat(events): add ReadFiltered with type/since/limit options"
```

### Sub-task 5B: `cleo events` command — basic path

Print events for an active session. No archive, no follow, no JSON. Get the table-format printer working first.

- [ ] **Step 5B.1: Write the failing test**

Create `internal/cli/events_test.go`:

```go
package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/dhruvsaxena1998/cleo/internal/events"
)

func TestEventsCmdPrintsActiveSession(t *testing.T) {
	c, root := testCtxWithRoot(t)
	defer os.Setenv("XDG_CONFIG_HOME", os.Getenv("XDG_CONFIG_HOME"))

	// Seed an event log
	log := events.NewLog(c.Paths.EventsLog("cleo-foo-claude-bar"))
	if err := log.Append(events.Entry{At: time.Now().UTC(), Type: "session_start"}); err != nil {
		t.Fatalf("seed: %v", err)
	}

	cmd := NewRootCmd(func(*Ctx) error { return nil })
	cmd.SetArgs([]string{"events", "cleo-foo-claude-bar", "-n", "10"})
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	os.Setenv("XDG_CONFIG_HOME", root)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if !strings.Contains(buf.String(), "session_start") {
		t.Errorf("output missing session_start: %q", buf.String())
	}

	// JSON mode passes through raw lines
	buf.Reset()
	cmd.SetArgs([]string{"events", "cleo-foo-claude-bar", "--json"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute json: %v", err)
	}
	var entry events.Entry
	if err := json.Unmarshal([]byte(strings.TrimSpace(buf.String())), &entry); err != nil {
		t.Errorf("json output not valid: %q (%v)", buf.String(), err)
	}
}

// testCtxWithRoot creates a Ctx rooted at a tempdir and returns (ctx, root).
// If the test file already has a similar helper, reuse it.
func testCtxWithRoot(t *testing.T) (*Ctx, string) {
	t.Helper()
	root := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", root)
	c, err := NewCtxWithRoot(filepath.Join(root, "cleo"))
	if err != nil {
		t.Fatalf("ctx: %v", err)
	}
	return c, filepath.Join(root, "cleo")
}
```

(If `testCtxWithRoot` collides with an existing helper in `cli_test_helpers_test.go`, rename and reuse the existing one.)

- [ ] **Step 5B.2: Run test, expect failure**

Run: `go test ./internal/cli/ -run TestEventsCmdPrintsActiveSession -v`
Expected: FAIL — `cleo events` command does not exist; cobra reports `unknown command "events"`.

- [ ] **Step 5B.3: Implement basic command**

Create `internal/cli/events.go`:

```go
package cli

import (
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"

	"github.com/dhruvsaxena1998/cleo/internal/events"
)

func newEventsCmd(getCtx func() *Ctx) *cobra.Command {
	var (
		follow bool
		typ    string
		since  string
		limit  int
		asJSON bool
	)
	cmd := &cobra.Command{
		Use:   "events <session-id>",
		Short: "Print or tail events for a session",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c := getCtx()
			id := args[0]
			path, archived, err := resolveEventsPath(c, id)
			if err != nil {
				return err
			}
			opts := events.ReadOpts{Type: typ, Limit: limit}
			if since != "" {
				d, err := time.ParseDuration(since)
				if err != nil {
					return fmt.Errorf("--since: %w", err)
				}
				opts.Since = time.Now().Add(-d)
			}
			if archived {
				return printArchivedEvents(cmd, path, opts, asJSON)
			}
			return printActiveEvents(cmd, path, opts, asJSON, follow)
		},
	}
	cmd.Flags().BoolVarP(&follow, "follow", "f", false, "tail the file (poll-based)")
	cmd.Flags().StringVar(&typ, "type", "", "filter to one event type (e.g. notification)")
	cmd.Flags().StringVar(&since, "since", "", "only events newer than now-<duration> (e.g. 15m)")
	cmd.Flags().IntVarP(&limit, "limit", "n", 0, "show only the most recent N events")
	cmd.Flags().BoolVar(&asJSON, "json", false, "emit raw JSONL lines")
	return cmd
}

// resolveEventsPath returns (path, archived, error).
// 1. Exact match on active events file.
// 2. Exact match on archived events file (.jsonl.gz).
// 3. Substring match across active+archived; error if multiple match.
func resolveEventsPath(c *Ctx, id string) (string, bool, error) {
	active := c.Paths.EventsLog(id)
	if _, err := os.Stat(active); err == nil {
		return active, false, nil
	}
	archived := filepath.Join(c.Paths.ArchiveDir(), id+".jsonl.gz")
	if _, err := os.Stat(archived); err == nil {
		return archived, true, nil
	}
	// Substring match: enumerate active and archived directories
	candidates, err := substringEventCandidates(c, id)
	if err != nil {
		return "", false, err
	}
	switch len(candidates) {
	case 0:
		return "", false, fmt.Errorf("unknown session: %s", id)
	case 1:
		return candidates[0].path, candidates[0].archived, nil
	default:
		var ids []string
		for _, c := range candidates {
			ids = append(ids, c.id)
		}
		return "", false, fmt.Errorf("ambiguous session %q matches: %s", id, strings.Join(ids, ", "))
	}
}

type eventCandidate struct {
	id       string
	path     string
	archived bool
}

func substringEventCandidates(c *Ctx, needle string) ([]eventCandidate, error) {
	var out []eventCandidate
	if entries, err := os.ReadDir(c.Paths.EventsDir()); err == nil {
		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			name := e.Name()
			if !strings.HasSuffix(name, ".jsonl") {
				continue
			}
			id := strings.TrimSuffix(name, ".jsonl")
			if strings.Contains(id, needle) {
				out = append(out, eventCandidate{id: id, path: filepath.Join(c.Paths.EventsDir(), name), archived: false})
			}
		}
	}
	if entries, err := os.ReadDir(c.Paths.ArchiveDir()); err == nil {
		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			name := e.Name()
			if !strings.HasSuffix(name, ".jsonl.gz") {
				continue
			}
			id := strings.TrimSuffix(name, ".jsonl.gz")
			if strings.Contains(id, needle) {
				out = append(out, eventCandidate{id: id, path: filepath.Join(c.Paths.ArchiveDir(), name), archived: true})
			}
		}
	}
	return out, nil
}

func printActiveEvents(cmd *cobra.Command, path string, opts events.ReadOpts, asJSON, follow bool) error {
	if asJSON {
		return streamJSONL(cmd.OutOrStdout(), path, follow)
	}
	log := events.NewLog(path)
	entries, err := log.ReadFiltered(opts)
	if err != nil {
		return err
	}
	if len(entries) == 0 {
		fmt.Fprintln(cmd.ErrOrStderr(), "(no events yet)")
		return nil
	}
	printEventsTable(cmd.OutOrStdout(), entries)
	if !follow {
		return nil
	}
	return tailEvents(cmd.OutOrStdout(), path, opts, asJSON)
}

func printArchivedEvents(cmd *cobra.Command, gzPath string, opts events.ReadOpts, asJSON bool) error {
	f, err := os.Open(gzPath)
	if err != nil {
		return err
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		return err
	}
	defer gz.Close()
	if asJSON {
		_, err := io.Copy(cmd.OutOrStdout(), gz)
		return err
	}
	// Decode line by line, apply filters, reuse table printer.
	dec := json.NewDecoder(gz)
	var entries []events.Entry
	for dec.More() {
		var e events.Entry
		if err := dec.Decode(&e); err != nil {
			break
		}
		if opts.Type != "" && e.Type != opts.Type {
			continue
		}
		if !opts.Since.IsZero() && e.At.Before(opts.Since) {
			continue
		}
		entries = append(entries, e)
	}
	if opts.Limit > 0 && len(entries) > opts.Limit {
		entries = entries[len(entries)-opts.Limit:]
	}
	printEventsTable(cmd.OutOrStdout(), entries)
	return nil
}

func printEventsTable(w io.Writer, entries []events.Entry) {
	dim := lipgloss.NewStyle().Foreground(lipgloss.Color("#6c7086"))
	for _, e := range entries {
		ts := dim.Render(e.At.Local().Format("15:04:05"))
		typ := e.Type
		msg := strings.TrimSpace(e.Detail)
		if msg == "" {
			msg = e.Tool
		}
		fmt.Fprintf(w, "%s\t%s\t%s\n", ts, typ, msg)
	}
}

func streamJSONL(w io.Writer, path string, follow bool) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	if _, err := io.Copy(w, f); err != nil {
		return err
	}
	if !follow {
		return nil
	}
	return tailRaw(w, path)
}

// tailEvents and tailRaw are defined in events_follow.go (sub-task 5C).
```

Add a stub for the `tailEvents` and `tailRaw` functions so the file compiles. We'll implement follow in sub-task 5C:

Append to `events.go`:

```go
func tailEvents(w io.Writer, path string, opts events.ReadOpts, asJSON bool) error {
	// Placeholder until sub-task 5C lands. -f without 5C just exits after the
	// initial dump.
	return nil
}

func tailRaw(w io.Writer, path string) error { return nil }
```

Register the command in `internal/cli/root.go`. Insert in the `root.AddCommand(...)` list after `newRenameCmd(getCtx)`:

```go
newEventsCmd(getCtx),
```

- [ ] **Step 5B.4: Run test, expect pass**

Run: `go test ./internal/cli/ -run TestEventsCmdPrintsActiveSession -v`
Expected: PASS.

- [ ] **Step 5B.5: Commit**

```bash
git add internal/cli/events.go internal/cli/events_test.go internal/cli/root.go
git commit -m "feat(cli): add cleo events <session-id> with type/since/limit/json"
```

### Sub-task 5C: `--follow` (tail) support

Poll the file every 500 ms and print appended lines. Reopen on inode change so a `cleo prune` archive doesn't kill the tail.

- [ ] **Step 5C.1: Write the failing test**

Append to `internal/cli/events_test.go`:

```go
func TestEventsCmdFollowEmitsAppendedLines(t *testing.T) {
	c, root := testCtxWithRoot(t)
	t.Setenv("XDG_CONFIG_HOME", root)
	logPath := c.Paths.EventsLog("cleo-foo-claude-bar")
	log := events.NewLog(logPath)
	if err := log.Append(events.Entry{At: time.Now().UTC(), Type: "session_start"}); err != nil {
		t.Fatalf("seed: %v", err)
	}

	cmd := NewRootCmd(func(*Ctx) error { return nil })
	cmd.SetArgs([]string{"events", "cleo-foo-claude-bar", "-f", "--json"})
	var buf bytes.Buffer
	cmd.SetOut(&buf)

	done := make(chan error, 1)
	go func() { done <- cmd.Execute() }()

	// Wait for initial dump
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) && !strings.Contains(buf.String(), "session_start") {
		time.Sleep(50 * time.Millisecond)
	}
	if !strings.Contains(buf.String(), "session_start") {
		t.Fatalf("initial dump missing: %q", buf.String())
	}

	// Append a second event, expect it to appear within ~1s
	if err := log.Append(events.Entry{At: time.Now().UTC(), Type: "post_tool_use"}); err != nil {
		t.Fatalf("append: %v", err)
	}
	deadline = time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) && !strings.Contains(buf.String(), "post_tool_use") {
		time.Sleep(100 * time.Millisecond)
	}
	if !strings.Contains(buf.String(), "post_tool_use") {
		t.Fatalf("appended event not seen: %q", buf.String())
	}

	// Cancel the follow by sending SIGINT — for the test we just stop reading.
	// In production a user hits Ctrl-C; here we let the test goroutine leak.
	_ = done
}
```

- [ ] **Step 5C.2: Run test, expect failure**

Run: `go test ./internal/cli/ -run TestEventsCmdFollowEmitsAppendedLines -v -timeout 30s`
Expected: FAIL — `tailRaw` is a stub that returns immediately.

- [ ] **Step 5C.3: Implement `tailRaw` and `tailEvents`**

Replace the stubs in `internal/cli/events.go`:

```go
func tailEvents(w io.Writer, path string, opts events.ReadOpts, asJSON bool) error {
	return tailLoop(w, path, func(line []byte) {
		if asJSON {
			fmt.Fprintln(w, string(line))
			return
		}
		var e events.Entry
		if err := json.Unmarshal(line, &e); err != nil {
			return
		}
		if opts.Type != "" && e.Type != opts.Type {
			return
		}
		if !opts.Since.IsZero() && e.At.Before(opts.Since) {
			return
		}
		printEventsTable(w, []events.Entry{e})
	})
}

func tailRaw(w io.Writer, path string) error {
	return tailLoop(w, path, func(line []byte) {
		fmt.Fprintln(w, string(line))
	})
}

// tailLoop polls path, calling onLine for each newly appended JSONL line.
// Reopens the file when the inode changes (so a prune→archive doesn't kill
// the follow). Caller is responsible for stopping the loop (Ctrl-C / process
// exit). The poll cadence is 500 ms.
func tailLoop(w io.Writer, path string, onLine func([]byte)) error {
	openFile := func() (*os.File, os.FileInfo, error) {
		f, err := os.Open(path)
		if err != nil {
			return nil, nil, err
		}
		st, err := f.Stat()
		if err != nil {
			f.Close()
			return nil, nil, err
		}
		// Seek to end; we already dumped initial contents in printActiveEvents.
		if _, err := f.Seek(0, io.SeekEnd); err != nil {
			f.Close()
			return nil, nil, err
		}
		return f, st, nil
	}

	f, st, err := openFile()
	if err != nil {
		return err
	}
	defer func() {
		if f != nil {
			f.Close()
		}
	}()

	buf := make([]byte, 0, 1<<20)
	tmp := make([]byte, 32*1024)
	for {
		n, err := f.Read(tmp)
		if n > 0 {
			buf = append(buf, tmp[:n]...)
			for {
				idx := indexByte(buf, '\n')
				if idx < 0 {
					break
				}
				onLine(buf[:idx])
				buf = buf[idx+1:]
			}
		}
		if err != nil && err != io.EOF {
			return err
		}
		// Sleep, then check for inode change
		time.Sleep(500 * time.Millisecond)
		newSt, statErr := os.Stat(path)
		if statErr == nil && !sameFile(st, newSt) {
			f.Close()
			f, st, err = openFile()
			if err != nil {
				return err
			}
			buf = buf[:0]
		}
	}
}

func indexByte(b []byte, c byte) int {
	for i := range b {
		if b[i] == c {
			return i
		}
	}
	return -1
}

func sameFile(a, b os.FileInfo) bool {
	if a == nil || b == nil {
		return false
	}
	return os.SameFile(a, b)
}
```

The test relies on the goroutine being abandoned at end of test — Go test framework handles process lifetime. If staticcheck flags the goroutine leak in CI later, gate the loop behind a `context.Context` accepted as a hidden parameter; for v0.2 the leak is harmless.

- [ ] **Step 5C.4: Run test, expect pass**

Run: `go test ./internal/cli/ -run TestEventsCmdFollowEmitsAppendedLines -v -timeout 30s`
Expected: PASS.

- [ ] **Step 5C.5: Commit**

```bash
git add internal/cli/events.go internal/cli/events_test.go
git commit -m "feat(cli): cleo events --follow tails JSONL with inode-change reopen"
```

### Sub-task 5D: Substring resolution + archive lookup

Verify the resolve/archive paths work end to end with one extra test.

- [ ] **Step 5D.1: Write the failing test**

Append to `internal/cli/events_test.go`:

```go
func TestEventsCmdResolvesSubstringAndArchive(t *testing.T) {
	c, root := testCtxWithRoot(t)
	t.Setenv("XDG_CONFIG_HOME", root)

	// Create one active session log
	activeID := "cleo-myapp-claude-active-thing"
	activeLog := events.NewLog(c.Paths.EventsLog(activeID))
	if err := activeLog.Append(events.Entry{At: time.Now().UTC(), Type: "session_start"}); err != nil {
		t.Fatalf("active seed: %v", err)
	}

	// Create one archived log via Archive helper
	archiveID := "cleo-myapp-claude-archived-thing"
	archiveSrc := c.Paths.EventsLog(archiveID)
	archiveLog := events.NewLog(archiveSrc)
	if err := archiveLog.Append(events.Entry{At: time.Now().UTC(), Type: "session_end"}); err != nil {
		t.Fatalf("archive seed: %v", err)
	}
	if err := events.Archive(archiveSrc, c.Paths.ArchiveDir()); err != nil {
		t.Fatalf("archive: %v", err)
	}

	// Substring match across both
	cmd := NewRootCmd(func(*Ctx) error { return nil })
	cmd.SetArgs([]string{"events", "active-thing", "-n", "10"})
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("active substring: %v", err)
	}
	if !strings.Contains(buf.String(), "session_start") {
		t.Errorf("active substring output: %q", buf.String())
	}

	// Archive substring match
	buf.Reset()
	cmd.SetArgs([]string{"events", "archived-thing", "-n", "10"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("archive substring: %v", err)
	}
	if !strings.Contains(buf.String(), "session_end") {
		t.Errorf("archive substring output: %q", buf.String())
	}

	// Ambiguous substring matches both → error
	buf.Reset()
	cmd.SetArgs([]string{"events", "myapp-claude", "-n", "10"})
	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "ambiguous") {
		t.Errorf("expected ambiguous error, got %v / output %q", err, buf.String())
	}
}
```

- [ ] **Step 5D.2: Run test, expect pass**

Run: `go test ./internal/cli/ -run TestEventsCmdResolvesSubstringAndArchive -v`
Expected: PASS — `resolveEventsPath` already implements this.

If it fails, debug the resolution logic before continuing.

- [ ] **Step 5D.3: Commit**

```bash
git add internal/cli/events_test.go
git commit -m "test(cli): cover substring resolution and archive lookup for events"
```

### Sub-task 5E: Update CHANGELOG and finish

- [ ] **Step 5E.1: Add entry under `[Unreleased]`**

Append under `### Added`:

```markdown
- `cleo events <session-id> [-f] [--type <kind>] [--since <duration>] [-n <limit>] [--json]` — print or tail per-session event logs. Supports substring matching and archived (`cleo prune`) sessions.
```

- [ ] **Step 5E.2: Final task verification**

Run: `go vet ./... && go test ./... -count=1`
Expected: PASS.

- [ ] **Step 5E.3: Commit**

```bash
git add CHANGELOG.md
git commit -m "docs(changelog): add cleo events command"
```

---

## Task 6: `cleo doctor` upgrade (spec §1.2)

**Goal:** Three new sections in `cleo doctor` plus `--quiet` flag. Consumes the `fallback_reason` field from Task 4.

**Files:**
- Modify: `internal/cli/doctor.go`
- Modify: `internal/cli/doctor_test.go`

### Sub-task 6A: Inline last 3 hook traces per agent

- [ ] **Step 6A.1: Write the failing test**

Append to `internal/cli/doctor_test.go`:

```go
func TestDoctorPrintsRecentTraces(t *testing.T) {
	dir := t.TempDir()
	tracePath := filepath.Join(dir, "hook-trace.log")
	rows := []string{
		`{"at":"2026-05-01T12:00:00Z","protocol":"claude","event":"SessionStart","resolved_session":"sid-a","result":"resolved","fallback_reason":"env_present"}`,
		`{"at":"2026-05-01T12:00:01Z","protocol":"claude","event":"PreToolUse","resolved_session":"sid-a","result":"resolved","fallback_reason":"env_present"}`,
		`{"at":"2026-05-01T12:00:02Z","protocol":"claude","event":"Stop","resolved_session":"sid-a","result":"resolved","fallback_reason":"env_present"}`,
		`{"at":"2026-05-01T12:00:03Z","protocol":"claude","event":"Notification","resolved_session":"sid-a","result":"resolved","fallback_reason":"env_present"}`,
	}
	if err := os.WriteFile(tracePath, []byte(strings.Join(rows, "\n")+"\n"), 0o644); err != nil {
		t.Fatalf("write trace: %v", err)
	}

	got := recentHookTraces(tracePath, "claude", 3)
	if len(got) != 3 {
		t.Fatalf("len: want 3, got %d", len(got))
	}
	if got[0].Event != "Notification" { // most recent first
		t.Errorf("ordering: want Notification first, got %s", got[0].Event)
	}
}
```

- [ ] **Step 6A.2: Run test, expect failure**

Run: `go test ./internal/cli/ -run TestDoctorPrintsRecentTraces -v`
Expected: FAIL — `recentHookTraces` undefined.

- [ ] **Step 6A.3: Implement**

In `internal/cli/doctor.go`, add:

```go
// recentHookTraces returns the n most recent trace rows for the given protocol,
// ordered most-recent-first.
func recentHookTraces(path, protocol string, n int) []hookTraceRow {
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()
	var rows []hookTraceRow
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		var row hookTraceRow
		if err := json.Unmarshal(scanner.Bytes(), &row); err != nil {
			continue
		}
		if row.Protocol == protocol {
			rows = append(rows, row)
		}
	}
	// Reverse to most-recent-first; truncate to n
	for i, j := 0, len(rows)-1; i < j; i, j = i+1, j-1 {
		rows[i], rows[j] = rows[j], rows[i]
	}
	if len(rows) > n {
		rows = rows[:n]
	}
	return rows
}
```

Wire it into `printDoctorReport` to print after each protocol's check. To know which check belongs to which protocol, change `doctorCheck` to optionally carry a protocol tag:

```go
type doctorCheck struct {
	Label    string
	OK       bool
	Detail   string
	Protocol string // "claude" | "codex" | "" — used to attach trace inline
}
```

Tag the relevant checks in `diagnoseHooks`:

```go
func diagnoseHooks(claudeSettingsPath, codexHooksPath, codexConfigPath, hookTracePath string) doctorReport {
	claude := checkClaudeHooks(claudeSettingsPath)
	claude.Protocol = "claude"
	codexFlag := checkCodexFeatureFlag(codexConfigPath)
	codexHooks := checkCodexHooks(codexHooksPath)
	codexHooks.Protocol = "codex"
	claudeAct := checkHookTrace(hookTracePath, "claude")
	claudeAct.Protocol = "claude"
	codexAct := checkHookTrace(hookTracePath, "codex")
	codexAct.Protocol = "codex"
	return doctorReport{Checks: []doctorCheck{claude, codexFlag, codexHooks, claudeAct, codexAct}}
}
```

Then `printDoctorReport` prints the recent-traces sub-block after the `*hook activity*` checks (the ones whose label contains "activity"):

```go
for _, check := range report.Checks {
	var symbol string
	if check.OK {
		symbol = okStyle.Render("✓")
	} else {
		symbol = warnStyle.Render("✗")
	}
	fmt.Fprintf(w, "%s %s: %s\n", symbol, check.Label, check.Detail)
	if strings.Contains(check.Label, "hook activity") && check.Protocol != "" {
		traces := recentHookTraces(report.HookTracePath, check.Protocol, 3)
		if len(traces) > 0 {
			fmt.Fprintln(w, "  Last hook traces:")
			for _, tr := range traces {
				ts := tr.At
				if t, err := time.Parse(time.RFC3339, tr.At); err == nil {
					ts = t.Local().Format("15:04:05")
				}
				fmt.Fprintf(w, "    %s  %-18s %-40s %s\n", ts, tr.Event, truncRight(tr.ResolvedSession, 40), tr.FallbackReason)
			}
		}
	}
}
```

`HookTracePath` is a new field on `doctorReport`:

```go
type doctorReport struct {
	Checks         []doctorCheck
	HookTracePath  string
}
```

Set it in `diagnoseHooks` (return statement), and pass it through. `truncRight` is a helper:

```go
func truncRight(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n-1] + "…"
}
```

- [ ] **Step 6A.4: Run test, expect pass**

Run: `go test ./internal/cli/ -run TestDoctorPrintsRecentTraces -v`
Expected: PASS.

- [ ] **Step 6A.5: Commit**

```bash
git add internal/cli/doctor.go internal/cli/doctor_test.go
git commit -m "feat(doctor): print last 3 hook traces inline per agent"
```

### Sub-task 6B: Attribution-failure summary

- [ ] **Step 6B.1: Write the failing test**

Append:

```go
func TestDoctorAttributionFailureSummary(t *testing.T) {
	dir := t.TempDir()
	tracePath := filepath.Join(dir, "hook-trace.log")
	rows := []string{
		`{"at":"2026-05-01T12:00:00Z","protocol":"codex","event":"PreToolUse","cwd":"/a","result":"resolved","fallback_reason":"env_missing"}`,
		`{"at":"2026-05-01T12:00:01Z","protocol":"codex","event":"PreToolUse","cwd":"/a","result":"ignored:no_session","fallback_reason":"no_match"}`,
		`{"at":"2026-05-01T12:00:02Z","protocol":"claude","event":"PreToolUse","result":"ignored:no_session","fallback_reason":"env_unknown_session"}`,
	}
	if err := os.WriteFile(tracePath, []byte(strings.Join(rows, "\n")+"\n"), 0o644); err != nil {
		t.Fatalf("write trace: %v", err)
	}

	failures := attributionFailures(tracePath, time.Time{})
	if len(failures) != 2 {
		t.Fatalf("len: want 2, got %d (%+v)", len(failures), failures)
	}
}
```

- [ ] **Step 6B.2: Run test, expect failure**

Run: `go test ./internal/cli/ -run TestDoctorAttributionFailureSummary -v`
Expected: FAIL — `attributionFailures` undefined.

- [ ] **Step 6B.3: Implement**

Add to `doctor.go`:

```go
// attributionFailures returns trace rows whose fallback_reason indicates
// resolution did not succeed (no_match or env_unknown_session). If `since`
// is non-zero, only rows newer than `since` are returned.
func attributionFailures(path string, since time.Time) []hookTraceRow {
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()
	var out []hookTraceRow
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		var row hookTraceRow
		if err := json.Unmarshal(scanner.Bytes(), &row); err != nil {
			continue
		}
		if row.FallbackReason != "no_match" && row.FallbackReason != "env_unknown_session" {
			continue
		}
		if !since.IsZero() {
			if t, err := time.Parse(time.RFC3339, row.At); err == nil && t.Before(since) {
				continue
			}
		}
		out = append(out, row)
	}
	return out
}
```

In `printDoctorReport`, after the per-check loop, add:

```go
since := time.Now().Add(-24 * time.Hour)
failures := attributionFailures(report.HookTracePath, since)
if len(failures) > 0 {
	fmt.Fprintln(w)
	fmt.Fprintf(w, "Attribution failures (last 24h): %d\n", len(failures))
	fmt.Fprintln(w, "  Last 3:")
	last := failures
	if len(last) > 3 {
		last = last[len(last)-3:]
	}
	for _, tr := range last {
		ts := tr.At
		if t, err := time.Parse(time.RFC3339, tr.At); err == nil {
			ts = t.Local().Format("15:04:05")
		}
		cwd := tr.Cwd
		if cwd == "" {
			cwd = "(no cwd)"
		}
		fmt.Fprintf(w, "    %s  %-30s %-18s %s\n", ts, truncRight(cwd, 30), tr.Event, tr.FallbackReason)
	}
}
```

- [ ] **Step 6B.4: Run test, expect pass**

Run: `go test ./internal/cli/ -run TestDoctorAttributionFailureSummary -v`
Expected: PASS.

- [ ] **Step 6B.5: Commit**

```bash
git add internal/cli/doctor.go internal/cli/doctor_test.go
git commit -m "feat(doctor): summarize recent attribution failures"
```

### Sub-task 6C: Hook-config diff

Compute `cleo init`'s expected hook entries vs. what's installed; print a +/- diff per agent.

- [ ] **Step 6C.1: Read what's expected**

Look at `internal/hooks/install.go` for the install logic. The expected entries are produced there; the diff function should call into the same source of truth (don't reimplement). Identify a function or set of functions that returns the expected entries per protocol — for instance, `hooks.ClaudeHookEntries(cleoBinaryPath)` or similar. If no such function exists, extract one as part of this sub-task: pull the entry-construction logic out of `Install` so both `Install` and `Diff` call it.

For the plan, assume the function will be named `hooks.ExpectedClaudeEntries(binPath string) map[string]any` and `hooks.ExpectedCodexEntries(binPath string) map[string]any`, returning the same shape that lives in the on-disk JSON.

- [ ] **Step 6C.2: Write the failing test**

Append:

```go
func TestDoctorHookConfigDiff(t *testing.T) {
	dir := t.TempDir()
	settings := filepath.Join(dir, "settings.json")
	expected := map[string]any{
		"hooks": map[string]any{
			"SessionStart": map[string]any{"command": "/path/to/cleo hook claude SessionStart"},
		},
	}
	b, _ := json.Marshal(expected)
	if err := os.WriteFile(settings, b, 0o644); err != nil {
		t.Fatalf("seed settings: %v", err)
	}

	// Diff against a richer expected set (UserPromptSubmit also expected)
	expectedEntries := map[string]any{
		"SessionStart":     map[string]any{"command": "/path/to/cleo hook claude SessionStart"},
		"UserPromptSubmit": map[string]any{"command": "/path/to/cleo hook claude UserPromptSubmit"},
	}

	d := hookConfigDiff(settings, expectedEntries)
	if !contains(d.matched, "SessionStart") {
		t.Errorf("matched should include SessionStart: %+v", d)
	}
	if !contains(d.toAdd, "UserPromptSubmit") {
		t.Errorf("toAdd should include UserPromptSubmit: %+v", d)
	}
}

func contains(xs []string, s string) bool {
	for _, x := range xs {
		if x == s {
			return true
		}
	}
	return false
}
```

- [ ] **Step 6C.3: Run test, expect failure**

Run: `go test ./internal/cli/ -run TestDoctorHookConfigDiff -v`
Expected: FAIL — `hookConfigDiff` undefined.

- [ ] **Step 6C.4: Implement**

Add to `doctor.go`:

```go
type hookDiff struct {
	matched   []string
	toAdd     []string
	conflicts []string // entries that exist but don't match cleo's expected command
}

func hookConfigDiff(settingsPath string, expectedEntries map[string]any) hookDiff {
	var d hookDiff
	b, err := os.ReadFile(settingsPath)
	if err != nil {
		// Treat as "all to add"
		for k := range expectedEntries {
			d.toAdd = append(d.toAdd, k)
		}
		sort.Strings(d.toAdd)
		return d
	}
	var settings map[string]any
	if err := json.Unmarshal(b, &settings); err != nil {
		for k := range expectedEntries {
			d.toAdd = append(d.toAdd, k)
		}
		sort.Strings(d.toAdd)
		return d
	}
	configured, _ := settings["hooks"].(map[string]any)
	for event, expected := range expectedEntries {
		actual, ok := configured[event]
		if !ok {
			d.toAdd = append(d.toAdd, event)
			continue
		}
		eb, _ := json.Marshal(expected)
		ab, _ := json.Marshal(actual)
		if string(eb) == string(ab) {
			d.matched = append(d.matched, event)
		} else {
			d.conflicts = append(d.conflicts, event)
		}
	}
	sort.Strings(d.matched)
	sort.Strings(d.toAdd)
	sort.Strings(d.conflicts)
	return d
}
```

Add `"sort"` to the imports if not present.

In `printDoctorReport`, after the attribution-failures block, print the diff for each agent. Skip if doctor has no way to know cleo's binary path (use `os.Executable()`).

- [ ] **Step 6C.5: Run test, expect pass**

Run: `go test ./internal/cli/ -run TestDoctorHookConfigDiff -v`
Expected: PASS.

- [ ] **Step 6C.6: Commit**

```bash
git add internal/cli/doctor.go internal/cli/doctor_test.go internal/hooks/install.go
git commit -m "feat(doctor): show installed-vs-expected hook config diff"
```

### Sub-task 6D: `--quiet` flag

- [ ] **Step 6D.1: Write the failing test**

Append:

```go
func TestDoctorQuietSuppressesPassingChecks(t *testing.T) {
	report := doctorReport{
		Checks: []doctorCheck{
			{Label: "Claude hooks", OK: true, Detail: "8 hooks installed"},
			{Label: "Codex feature flag", OK: false, Detail: "missing"},
		},
	}
	var buf bytes.Buffer
	printDoctorReportOpts(&buf, report, doctorPrintOpts{Quiet: true})
	out := buf.String()
	if strings.Contains(out, "Claude hooks") {
		t.Errorf("quiet mode should hide passing check, got %q", out)
	}
	if !strings.Contains(out, "Codex feature flag") {
		t.Errorf("quiet mode should still show failure, got %q", out)
	}
}
```

- [ ] **Step 6D.2: Run test, expect failure**

Run: `go test ./internal/cli/ -run TestDoctorQuietSuppressesPassingChecks -v`
Expected: FAIL — `printDoctorReportOpts` / `doctorPrintOpts` undefined.

- [ ] **Step 6D.3: Implement**

In `doctor.go`, replace `printDoctorReport` with:

```go
type doctorPrintOpts struct {
	Quiet bool
}

func printDoctorReport(w io.Writer, report doctorReport) {
	printDoctorReportOpts(w, report, doctorPrintOpts{})
}

func printDoctorReportOpts(w io.Writer, report doctorReport, opts doctorPrintOpts) {
	if !opts.Quiet {
		fmt.Fprintln(w, "Cleo doctor")
		fmt.Fprintln(w)
	}
	for _, check := range report.Checks {
		if opts.Quiet && check.OK {
			continue
		}
		var symbol string
		if check.OK {
			symbol = okStyle.Render("✓")
		} else {
			symbol = warnStyle.Render("✗")
		}
		fmt.Fprintf(w, "%s %s: %s\n", symbol, check.Label, check.Detail)
		// (recent-traces inline block from 6A.3 lives here)
	}
	// (attribution-failures block from 6B.3 lives here, omitted when no failures)
	// (hook-config diff from 6C lives here, omitted when in sync)
	if !opts.Quiet {
		fmt.Fprintln(w)
		fmt.Fprintln(w, "Codex approval check:")
		// ... existing trailer
	}
}
```

Add the `--quiet` flag in `newDoctorCmd`:

```go
var quiet bool
cmd := &cobra.Command{
	// ...
	RunE: func(cmd *cobra.Command, args []string) error {
		c := getCtx()
		// ... existing report build
		printDoctorReportOpts(cmd.OutOrStdout(), report, doctorPrintOpts{Quiet: quiet})
		return nil
	},
}
cmd.Flags().BoolVar(&quiet, "quiet", false, "only print failures and non-empty diagnostic sections")
```

Exit code: when `--quiet` is set and any check failed (or attribution failures > 0, or diff non-empty), exit 1. Otherwise 0.

- [ ] **Step 6D.4: Run test, expect pass**

Run: `go test ./internal/cli/ -run TestDoctorQuietSuppressesPassingChecks -v`
Expected: PASS.

- [ ] **Step 6D.5: Update CHANGELOG**

Under `### Added`:

```markdown
- `cleo doctor` now prints recent hook traces, an attribution-failure summary (last 24h), and a +/- diff between expected and installed hook entries. New `--quiet` flag suppresses passing checks for cron use.
```

- [ ] **Step 6D.6: Commit**

```bash
git add internal/cli/doctor.go internal/cli/doctor_test.go CHANGELOG.md
git commit -m "feat(doctor): --quiet flag for cron-friendly output"
```

### Sub-task 6E: Extend `scripts/smoke.sh` (spec §5.2)

The smoke script gains the new commands. Stays opt-in (not run in CI).

- [ ] **Step 6E.1: Edit `scripts/smoke.sh`**

Append, before the existing `$bin kill ...` line:

```bash
$bin events cleo-tmp-claude-smoke -n 5
$bin doctor
$bin doctor --quiet >/dev/null
```

The `--quiet` line redirects stdout because, on a healthy install, quiet mode prints nothing — but exit code 0 confirms the command ran. If the install is dirty (failures or attribution issues), output goes to stdout/stderr and the smoke script will surface it.

- [ ] **Step 6E.2: Verify**

Run: `./scripts/smoke.sh`
Expected: all three commands run without error; the script prints `smoke OK` at the end. Requires a real `claude` CLI and tmux installed locally.

- [ ] **Step 6E.3: Commit**

```bash
git add scripts/smoke.sh
git commit -m "test: extend smoke script with cleo events and cleo doctor checks"
```

---

## Task 7: Pane preview reliability rewrite (spec §2.1)

**Goal:** Six concrete bugs in the pane preview pipeline. The biggest visible v0.2 change.

**Files:**
- Modify: `internal/tmux/tmux.go` (Bug A)
- Modify: `internal/tmux/tmux_test.go` (Bug A test)
- Modify: `internal/tui/poll.go` (Bug B)
- Modify: `internal/tui/update.go` (Bug B)
- Modify: `internal/tui/model.go` (Bug B + paneCaptureInFlight field)
- Modify: `internal/tui/main_pane.go` (Bugs D, E)
- Modify: `internal/tui/tui_test.go` (Bug B test)

### Sub-task 7A: `CapturePane` consumes the `lines` parameter (Bug A)

- [ ] **Step 7A.1: Write the failing test**

Append to `internal/tmux/tmux_test.go`. The strategy: shell out to `tmux` itself isn't worth automating; instead test that the constructed argv contains `-S -<lines>`. To make this testable, refactor `CapturePane` to build its argv via a helper that returns `[]string`, then unit-test the helper:

```go
func TestCapturePaneArgsIncludeScrollbackFlag(t *testing.T) {
	args := capturePaneArgs("cleo-foo", 50)
	want := []string{"capture-pane", "-p", "-S", "-50", "-t", "cleo-foo:."}
	if !equalStrings(args, want) {
		t.Errorf("argv: want %v, got %v", want, args)
	}

	// Default fallback when lines <= 0
	args = capturePaneArgs("cleo-bar", 0)
	if args[3] != "-30" {
		t.Errorf("default lines: want -30, got %s", args[3])
	}
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
```

- [ ] **Step 7A.2: Run test, expect failure**

Run: `go test ./internal/tmux/ -run TestCapturePaneArgsIncludeScrollbackFlag -v`
Expected: FAIL — `capturePaneArgs` undefined.

- [ ] **Step 7A.3: Implement**

In `internal/tmux/tmux.go`:

```go
func capturePaneArgs(name string, lines int) []string {
	if lines <= 0 {
		lines = 30
	}
	return []string{"capture-pane", "-p", "-S", fmt.Sprintf("-%d", lines), "-t", name + ":."}
}

func (c *Client) CapturePane(name string, lines int) (string, error) {
	out, err := c.cmd(capturePaneArgs(name, lines)...).Output()
	return string(out), err
}
```

Add `"fmt"` to imports if not already present.

- [ ] **Step 7A.4: Run test, expect pass**

Run: `go test ./internal/tmux/ -run TestCapturePaneArgsIncludeScrollbackFlag -v`
Expected: PASS.

- [ ] **Step 7A.5: Commit**

```bash
git add internal/tmux/tmux.go internal/tmux/tmux_test.go
git commit -m "fix(tmux): CapturePane respects pane_preview_lines via -S flag"
```

### Sub-task 7B: Selection-driven preview ticker (Bug B + Bug C + Bug F)

Replace the response→tick chain with one ticker that decides what to capture per fire.

- [ ] **Step 7B.1: Write the failing test**

Append to `internal/tui/tui_test.go`. Strategy: fire `previewTickMsg` repeatedly while the model's selection changes; assert the ticker always re-arms with another `previewTickMsg`.

```go
func TestPreviewTickAlwaysReArms(t *testing.T) {
	c := newFakeCtxForTest(t) // returns *cli.Ctx with fake tmux + state
	m := New(c)
	m.sessions = []state.Session{{ID: "s1", State: state.Running}}
	m.cursor.projectIdx = 0
	m.cursor.agentIdx = 0
	m.expanded = map[string]bool{}
	// (Helper newFakeCtxForTest may need to be added; see existing tui_test.go for analogous helpers.)

	// Fire a tick. The returned cmd should produce another previewTickMsg
	// after the configured interval, regardless of selection state.
	updated, cmd := m.Update(previewTickMsg{})
	_ = updated
	if cmd == nil {
		t.Fatal("previewTickMsg should return a non-nil command (re-arm + maybe capture)")
	}
	// Drain the batch, find a previewTickMsg-producing tick.
	msgs := drainBatchedTicks(cmd)
	found := false
	for _, m := range msgs {
		if _, ok := m.(previewTickMsg); ok {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected a re-arming previewTickMsg in the batch")
	}
}

// drainBatchedTicks runs cmd until it produces concrete tea.Msg values, then
// returns them. Tea internals expose this via a debug helper in some test
// suites; if not available, simulate by calling cmd() — for tea.Tick it'll
// block until time elapses, which is too slow. Use teatest's Drain or hand-roll
// a minimal version that recognizes tea.Batch and tea.Tick. See
// charmbracelet/x/exp/teatest for an example. If the harness isn't available,
// rewrite the assertion: invoke cmd in a goroutine with a short interval (use
// a test-only override of m.ctx.Config.UI.PanePreviewInterval = 50ms), then
// wait for the message on a channel.
```

The drain helper is non-trivial. Practical alternative: lower `PanePreviewInterval` for the test, run `cmd()` in a goroutine, read the message it produces:

```go
func TestPreviewTickAlwaysReArms(t *testing.T) {
	c := newFakeCtxForTest(t)
	c.Config.UI.PanePreviewInterval = 10 * time.Millisecond
	m := New(c)
	m.sessions = []state.Session{{ID: "s1", State: state.Running}}
	m.projects = []projects.Project{{ID: "p"}}
	m.cursor.projectIdx = 0
	m.cursor.agentIdx = 0
	m.expanded = map[string]bool{"p": true}
	// Wire a sessionsFor that returns m.sessions for project "p"; if absent in
	// the test path, fake the relationship by setting ProjectID on the session.
	m.sessions[0].ProjectID = "p"

	_, cmd := m.Update(previewTickMsg{})
	if cmd == nil {
		t.Fatal("expected non-nil cmd from previewTickMsg")
	}
	// cmd is a tea.Batch of [previewTickCmd, capturePaneCmd]. Run them with a
	// timeout and look for a previewTickMsg.
	out := runCmdAndCollect(t, cmd, 200*time.Millisecond)
	if !containsType(out, previewTickMsg{}) {
		t.Errorf("expected previewTickMsg in output, got %T", out)
	}
}

// runCmdAndCollect runs cmd to completion (or timeout) and returns each tea.Msg
// produced. tea.Batch is unwrapped one level. Adapt as needed.
func runCmdAndCollect(t *testing.T, cmd tea.Cmd, timeout time.Duration) []tea.Msg {
	t.Helper()
	if cmd == nil {
		return nil
	}
	ch := make(chan tea.Msg, 8)
	go func() {
		msg := cmd()
		// If batch, send each child by reflection or by re-running.
		if batch, ok := msg.(tea.BatchMsg); ok {
			for _, sub := range batch {
				if sub != nil {
					ch <- sub()
				}
			}
		} else {
			ch <- msg
		}
		close(ch)
	}()
	var out []tea.Msg
	deadline := time.After(timeout)
	for {
		select {
		case m, ok := <-ch:
			if !ok {
				return out
			}
			out = append(out, m)
		case <-deadline:
			return out
		}
	}
}

func containsType(msgs []tea.Msg, want tea.Msg) bool {
	for _, m := range msgs {
		if reflect.TypeOf(m) == reflect.TypeOf(want) {
			return true
		}
	}
	return false
}
```

If `newFakeCtxForTest` doesn't already exist in `tui_test.go`, copy the pattern from existing tests; the snapshot test in `tui_test.go` builds a context — replicate.

- [ ] **Step 7B.2: Run test, expect failure**

Run: `go test ./internal/tui/ -run TestPreviewTickAlwaysReArms -v`
Expected: FAIL — `previewTickMsg` undefined; the model still uses the old `paneCapturedMsg → tea.Tick → capturePaneTickMsg` chain.

- [ ] **Step 7B.3: Implement the new ticker**

In `internal/tui/poll.go`:

```go
type previewTickMsg struct{}

func previewTickCmd(interval time.Duration) tea.Cmd {
	if interval <= 0 {
		interval = 1500 * time.Millisecond
	}
	return tea.Tick(interval, func(time.Time) tea.Msg { return previewTickMsg{} })
}
```

Delete `capturePaneTickMsg` and the `tickStateCmd`-like wrappers around it.

In `internal/tui/model.go`, add to `Model`:

```go
type Model struct {
	// ... existing fields
	paneCaptureInFlight bool
}
```

Update `Init`:

```go
func (m Model) Init() tea.Cmd {
	return tea.Batch(
		loadStateCmd(m.ctx),
		tickStateCmd(),
		previewTickCmd(m.ctx.Config.UI.PanePreviewInterval),
	)
}
```

In `internal/tui/update.go`:

Replace the `paneCapturedMsg` handler with:

```go
case paneCapturedMsg:
	if m.paneCache == nil {
		m.paneCache = map[string]string{}
	}
	m.paneCache[msg.sid] = msg.content
	m.paneCaptureInFlight = false
	return m, nil
```

Delete the `capturePaneTickMsg` case entirely.

Add a new case for `previewTickMsg`:

```go
case previewTickMsg:
	next := previewTickCmd(m.ctx.Config.UI.PanePreviewInterval)
	if !m.ctx.Config.UI.ShowPanePreview {
		return m, next
	}
	sess, ok := m.selectedSession()
	if !ok || sess.State.IsFinished() || m.paneCaptureInFlight {
		return m, next
	}
	m.paneCaptureInFlight = true
	return m, tea.Batch(next, capturePaneCmd(m.ctx, sess.ID, m.ctx.Config.UI.PanePreviewLines))
```

`autoCaptureCmd` (used on cursor up/down) stays as-is — it provides immediate captures on selection change. But guard it to also set `paneCaptureInFlight`:

```go
func (m Model) autoCaptureCmd() tea.Cmd {
	sess, ok := m.sessionAtCursor()
	if !ok {
		return nil
	}
	if sess.State.IsFinished() {
		return nil
	}
	if m.paneCaptureInFlight {
		return nil
	}
	// Cannot mutate m here (autoCaptureCmd takes value receiver and returns cmd);
	// the in-flight flag will be set by callers that already mutate m. For now,
	// allow potential overlap on rapid navigation — the worst case is one extra
	// shell-out, harmless.
	return capturePaneCmd(m.ctx, sess.ID, m.ctx.Config.UI.PanePreviewLines)
}
```

(Setting the flag in `autoCaptureCmd` requires changing its signature; for v0.2, accept the rare overlap.)

- [ ] **Step 7B.4: Run test, expect pass**

Run: `go test ./internal/tui/ -run TestPreviewTickAlwaysReArms -v`
Expected: PASS.

- [ ] **Step 7B.5: Run all tui tests**

Run: `go test ./internal/tui/ -count=1 -v`
Expected: PASS. Existing tests may have asserted on the old `capturePaneTickMsg` chain — update to assert on `previewTickMsg`.

- [ ] **Step 7B.6: Commit**

```bash
git add internal/tui/poll.go internal/tui/update.go internal/tui/model.go internal/tui/tui_test.go
git commit -m "fix(tui): selection-driven preview ticker that cannot deadlock

Previous design chained paneCapturedMsg -> tea.Tick -> capturePaneTickMsg.
If the user navigated between fire and response, the response sid no longer
matched the selection, the chain returned m, nil, and the loop died until
the next user navigation. Replace with a single previewTickCmd that always
re-arms and decides per tick what to capture based on current selection."
```

### Sub-task 7C: Width truncation in render (Bug D)

- [ ] **Step 7C.1: Write the failing test**

Append to `internal/tui/tui_test.go`:

```go
func TestPreviewLinesAreTruncatedToPanelWidth(t *testing.T) {
	c := newFakeCtxForTest(t)
	m := New(c)
	long := strings.Repeat("X", 200)
	m.paneCache = map[string]string{"s1": long + "\nshort\n"}
	m.sessions = []state.Session{{ID: "s1", State: state.Running, ProjectID: "p"}}
	m.cursor.projectIdx = 0
	m.cursor.agentIdx = 0
	m.expanded = map[string]bool{"p": true}
	m.projects = []projects.Project{{ID: "p"}}
	m.width, m.height = 80, 30

	out := m.renderPreviewPanel(40, 20, m.sessions[0], true)
	// Expect every visible line to be at most ~36 cells wide (panel padding ~4).
	for _, line := range strings.Split(out, "\n") {
		if visualWidth(line) > 40 {
			t.Errorf("line wider than panel: %q (%d cells)", line, visualWidth(line))
		}
	}
}

// visualWidth ignores ANSI sequences. Use lipgloss.Width if available.
func visualWidth(s string) int { return lipgloss.Width(s) }
```

- [ ] **Step 7C.2: Run test, expect failure**

Run: `go test ./internal/tui/ -run TestPreviewLinesAreTruncatedToPanelWidth -v`
Expected: FAIL — long line is rendered without truncation.

- [ ] **Step 7C.3: Implement**

In `internal/tui/main_pane.go`, in `renderPreviewPanel`, change the body construction:

```go
body := make([]string, len(shown))
for i, l := range shown {
	body[i] = dimmed.Render(truncateWidth(l, w-4))
}
```

- [ ] **Step 7C.4: Run test, expect pass**

Run: `go test ./internal/tui/ -run TestPreviewLinesAreTruncatedToPanelWidth -v`
Expected: PASS.

- [ ] **Step 7C.5: Commit**

```bash
git add internal/tui/main_pane.go internal/tui/tui_test.go
git commit -m "fix(tui): truncate preview lines to panel inner width"
```

### Sub-task 7D: Empty / whitespace fallback messaging (Bug E)

- [ ] **Step 7D.1: Write the failing test**

Append:

```go
func TestPreviewWhitespaceShowsAttachHint(t *testing.T) {
	c := newFakeCtxForTest(t)
	m := New(c)
	m.paneCache = map[string]string{"s1": "   \n  \n"}
	sess := state.Session{ID: "s1", State: state.Running}
	out := m.renderPreviewPanel(60, 10, sess, true)
	if !strings.Contains(out, "press Enter to attach") {
		t.Errorf("expected attach hint for whitespace-only pane, got: %q", out)
	}
}

func TestPreviewEmptyShowsLoading(t *testing.T) {
	c := newFakeCtxForTest(t)
	m := New(c)
	m.paneCache = map[string]string{}
	sess := state.Session{ID: "s1", State: state.Running}
	out := m.renderPreviewPanel(60, 10, sess, true)
	if !strings.Contains(out, "loading") {
		t.Errorf("expected loading hint for empty cache, got: %q", out)
	}
}
```

- [ ] **Step 7D.2: Run test, expect failure**

Run: `go test ./internal/tui/ -run TestPreviewWhitespace -v`
Expected: FAIL — current code shows "loading… press v to refresh" for both.

- [ ] **Step 7D.3: Implement**

In `internal/tui/main_pane.go`, replace the `if pane == ""` block:

```go
pane := m.paneCache[sess.ID]
hint := "tmux capture-pane -p"
switch {
case pane == "":
	return m.theme.PanelBox("Terminal Preview", hint,
		[]string{faint.Render("loading…")}, w, h)
case strings.TrimSpace(pane) == "":
	return m.theme.PanelBox("Terminal Preview", hint,
		[]string{faint.Render("agent hasn't rendered yet — press Enter to attach")}, w, h)
}
```

- [ ] **Step 7D.4: Run tests, expect pass**

Run: `go test ./internal/tui/ -run TestPreview -v`
Expected: PASS.

- [ ] **Step 7D.5: Update CHANGELOG**

Under `### Fixed`:

```markdown
- Pane preview no longer freezes after rapid navigation. Preview ticker is now selection-driven and self-recovering.
- `pane_preview_lines` is honored (was silently ignored in v0.1).
- Long captured lines are truncated to the panel width instead of wrapping and breaking the layout.
- Whitespace-only pane (e.g. `--no-attach` agent) shows an attach hint instead of an unhelpful "loading…".
```

- [ ] **Step 7D.6: Commit**

```bash
git add internal/tui/main_pane.go internal/tui/tui_test.go CHANGELOG.md
git commit -m "fix(tui): distinguish empty-cache and whitespace-only pane states"
```

---

## Task 8: Esc / filter / status UX pass (spec §2.2)

**Goal:** Esc has a predictable hierarchy. Filter survives expand/collapse. Status auto-clears on user input.

**Files:**
- Modify: `internal/tui/handle_key.go`
- Modify: `internal/tui/filter.go`
- Modify: `internal/tui/model.go`
- Modify: `internal/tui/tui_test.go`

### Sub-task 8A: Esc hierarchy

- [ ] **Step 8A.1: Write the failing test**

Append:

```go
func TestEscClosesPopupOnly(t *testing.T) {
	c := newFakeCtxForTest(t)
	m := New(c)
	m.popup = NewHelpPopup(m.theme)
	m.mode = ModePopup
	m.filter = "active-query"
	m.status = "stale-status"

	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc}).(Model), nil
	// Translate: Update returns (tea.Model, tea.Cmd); helper to assert Model

	// After Esc on a popup: popup closed, status cleared, filter intact.
	m2 := updateAsModel(m, tea.KeyMsg{Type: tea.KeyEsc})
	if m2.popup != nil {
		t.Error("popup should be closed")
	}
	if m2.status != "" {
		t.Errorf("status should be cleared, got %q", m2.status)
	}
	if m2.filter != "active-query" {
		t.Errorf("filter should survive popup close, got %q", m2.filter)
	}
}

func TestEscInFilterClearsQueryAndExits(t *testing.T) {
	c := newFakeCtxForTest(t)
	m := New(c)
	m.mode = ModeFilter
	m.filter = "search"
	m2 := updateAsModel(m, tea.KeyMsg{Type: tea.KeyEsc})
	if m2.mode != ModeNormal {
		t.Errorf("mode: want Normal, got %v", m2.mode)
	}
	if m2.filter != "" {
		t.Errorf("filter: want empty, got %q", m2.filter)
	}
}

func TestEscInNormalClearsStatus(t *testing.T) {
	c := newFakeCtxForTest(t)
	m := New(c)
	m.mode = ModeNormal
	m.status = "old"
	m2 := updateAsModel(m, tea.KeyMsg{Type: tea.KeyEsc})
	if m2.status != "" {
		t.Errorf("status: want empty, got %q", m2.status)
	}
}

// updateAsModel runs Update and asserts the resulting tea.Model is a *Model.
func updateAsModel(m Model, msg tea.Msg) Model {
	out, _ := m.Update(msg)
	return out.(Model)
}
```

- [ ] **Step 8A.2: Run tests, expect failure**

Run: `go test ./internal/tui/ -run TestEsc -v`
Expected: FAIL — current handler does not implement the hierarchy.

- [ ] **Step 8A.3: Implement**

In `internal/tui/handle_key.go`, at the top of `handleKey`, add the Esc hierarchy before the existing dispatch:

```go
func (m Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if msg.Type == tea.KeyEsc {
		if m.mode == ModePopup && m.popup != nil {
			m.popup = nil
			m.mode = ModeNormal
			m.status = ""
			return m, nil
		}
		if m.mode == ModeFilter {
			m.mode = ModeNormal
			m.filter = ""
			return m.clampCursor(), nil
		}
		m.status = ""
		return m, nil
	}
	if m.mode == ModeFilter {
		return m.handleFilterKey(msg)
	}
	if m.mode == ModePopup && m.popup != nil {
		// ... existing popup forward
	}
	// ... existing dispatch
}
```

The change to `handleFilterKey` in `internal/tui/filter.go`: remove the `tea.KeyEsc` case (now handled centrally):

```go
func (m Model) handleFilterKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEnter:
		m.mode = ModeNormal
		return m.clampCursor(), nil
	case tea.KeyBackspace:
		// ...
	}
	// ...
}
```

- [ ] **Step 8A.4: Run tests, expect pass**

Run: `go test ./internal/tui/ -run TestEsc -v`
Expected: PASS.

- [ ] **Step 8A.5: Commit**

```bash
git add internal/tui/handle_key.go internal/tui/filter.go internal/tui/tui_test.go
git commit -m "feat(tui): explicit Esc hierarchy (popup -> filter -> status)"
```

### Sub-task 8B: Filter persists across expand/collapse

- [ ] **Step 8B.1: Write the failing test**

Append:

```go
func TestFilterSurvivesExpandCollapse(t *testing.T) {
	c := newFakeCtxForTest(t)
	m := New(c)
	m.projects = []projects.Project{{ID: "p1"}, {ID: "p2"}}
	m.sessions = []state.Session{{ID: "s1", ProjectID: "p1", Name: "alpha"}}
	m.filter = "alpha"
	m.expanded = map[string]bool{"p1": false}

	m2, _ := m.toggleExpand()
	if m2.filter != "alpha" {
		t.Errorf("filter cleared on expand: got %q", m2.filter)
	}
}
```

- [ ] **Step 8B.2: Run test, expect pass or failure depending on current code**

Run: `go test ./internal/tui/ -run TestFilterSurvivesExpandCollapse -v`
Most likely PASS — toggleExpand doesn't currently touch m.filter. Confirm by reading `toggleExpand` in `handle_key.go:95`. If it passes, the lock-in test alone is the deliverable.

- [ ] **Step 8B.3: Verify cursor follows session by ID**

Append a test asserting `clampCursor` keeps the cursor on a session by ID after row reflow. If this requires the new `cursorOnSessionID` helper, write it as part of this sub-task; otherwise just lock current behavior. Read `clampCursor` first to decide.

- [ ] **Step 8B.4: Commit**

```bash
git add internal/tui/tui_test.go
git commit -m "test(tui): lock filter persistence across expand/collapse"
```

### Sub-task 8C: Status auto-clear on user input

- [ ] **Step 8C.1: Write the failing test**

Append:

```go
func TestStatusClearsOnExpand(t *testing.T) {
	c := newFakeCtxForTest(t)
	m := New(c)
	m.projects = []projects.Project{{ID: "p1"}}
	m.cursor.projectIdx = 0
	m.status = "old"

	m2, _ := m.toggleExpand()
	if m2.status != "" {
		t.Errorf("status should clear on expand, got %q", m2.status)
	}
}

func TestStatusClearsOnFilterEntry(t *testing.T) {
	c := newFakeCtxForTest(t)
	m := New(c)
	m.status = "old"
	m2 := updateAsModel(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
	// Entering filter mode should clear status.
	// (Depending on keymap, '/' triggers ModeFilter via Filter action.)
	_ = m2
	// Test passes if any path that enters ModeFilter clears status.
	// If the implementation hooks status-clear to the entry-key handler in
	// handleKey rather than via a generic clear, write the assertion against
	// the resulting model.
}
```

- [ ] **Step 8C.2: Run test, expect failure**

Run: `go test ./internal/tui/ -run TestStatusClearsOn -v`
Expected: `TestStatusClearsOnExpand` FAIL — `toggleExpand` already does `m.status = ""` (per existing code), so this may PASS. Read `toggleExpand` to confirm. If it does, this is a lock-in test; commit it. If not, add the line.

- [ ] **Step 8C.3: Implement (if needed)**

If any user-action path doesn't clear status, add `m.status = ""` at the top of that function. Specifically check:
- `toggleExpand` ✓ (already)
- `cursorUp`, `cursorDown` ✓ (already, per v0.1 commit history)
- `openSpawnPopup`, `openRenamePopup`, `openHelpPopup`, `confirmKill` — add `m.status = ""` if missing.
- Filter mode entry (the `case key.Matches(msg, km.Filter):` branch in `handleKey`): add `m.status = ""` before `m.mode = ModeFilter`.

- [ ] **Step 8C.4: Run tests, expect pass**

Run: `go test ./internal/tui/ -run TestStatusClearsOn -v`
Expected: PASS.

- [ ] **Step 8C.5: Update CHANGELOG**

Under `### Changed`:

```markdown
- Esc has a predictable hierarchy: closes the active popup, then exits filter mode (clearing the query), then clears the status line. `q` no longer quits while inside a popup or filter.
- Status line clears on any user-initiated state change (cursor move, expand/collapse, popup open, filter entry), not just navigation.
```

- [ ] **Step 8C.6: Commit**

```bash
git add internal/tui/handle_key.go internal/tui/tui_test.go CHANGELOG.md
git commit -m "feat(tui): status line auto-clears on every user-initiated change"
```

---

## Task 9: CONTRIBUTING.md + bug-report issue template (spec §4.3)

**Goal:** Document how to contribute and provide a structured bug-report issue form. No code, no tests.

**Files:**
- Create: `CONTRIBUTING.md`
- Create: `.github/ISSUE_TEMPLATE/bug_report.yml`

- [ ] **Step 9.1: Write CONTRIBUTING.md**

```markdown
# Contributing to cleo

cleo is in alpha (v0.2 at time of writing). Behavior, configuration, and CLI surface may change between minor releases. Bug reports and small PRs are welcome; for larger changes, open an issue first to discuss the design.

## Filing issues

- Bug: use the [bug report form](https://github.com/dhruvsaxena1998/cleo/issues/new?template=bug_report.yml).
- Idea or feature: open a regular issue describing the use case.

## Building and testing

```bash
make build         # go build -o bin/cleo ./cmd/cleo
make test          # go test ./...
make lint          # go vet ./...
make run           # build and launch ./bin/cleo
./scripts/smoke.sh # end-to-end manual smoke; requires claude CLI and tmux
```

## Test policy

- Every new feature ships with at least one test.
- Every bug fix ships with a regression test that fails on the broken code and passes on the fix.
- TUI rendering and tmux interactions that resist unit testing are covered by manual rituals (below).

## Commit format

Conventional Commits (`feat:`, `fix:`, `chore:`, `docs:`, `refactor:`, `ci:`, `test:`). Subject under 70 characters; body explains the *why* if non-obvious.

## Pull requests

- Keep PRs small and focused (≤ ~400 lines diff is a soft target).
- Describe the *why* in the PR body: what problem, what trade-offs.
- Add a one-line entry under `## [Unreleased]` in `CHANGELOG.md` in the same commit as the code change.

## Manual verification rituals

Some behaviors aren't fully covered by unit tests. Run these by hand before approving PRs that touch the relevant area.

### Pane preview correctness (touches `internal/tui/`, `internal/tmux/CapturePane`)

Open `cleo` against a project with at least one running claude session and one running codex session. Navigate up/down between them rapidly (10+ alternations). The preview must not get stuck on either session's last frame; the visual content for the currently-selected row must update within `pane_preview_interval`. Resize the terminal to roughly half-width — panel borders must stay aligned across all rows.

### Reconciler timing (touches `internal/reconcile/`, `internal/state/`)

Set `idle_to_completed_timeout = 30s`, `spawning_timeout = 10s` in test config. Start a session, immediately fire `Notification`, observe the TUI state column for ~70s. Expected progression: `Spawning → Running → WaitingForInput → Idle → Completed`.

## License

cleo is MIT-licensed. By contributing, you agree your contributions will be MIT-licensed too.
```

- [ ] **Step 9.2: Write the issue template**

Create `.github/ISSUE_TEMPLATE/bug_report.yml`:

```yaml
name: Bug report
description: Report a bug in cleo
title: "[bug] "
labels: ["bug", "triage"]
body:
  - type: input
    id: version
    attributes:
      label: cleo version
      description: Output of `cleo --version`
      placeholder: "cleo version 0.2.0"
    validations:
      required: true

  - type: input
    id: os
    attributes:
      label: OS and tmux version
      description: e.g. macOS 14.4 / `tmux 3.4`
    validations:
      required: true

  - type: textarea
    id: steps
    attributes:
      label: Steps to reproduce
      description: What you did, in order. Include the exact `cleo` commands.
    validations:
      required: true

  - type: textarea
    id: expected
    attributes:
      label: Expected vs actual
      description: What you expected to happen, and what actually happened.
    validations:
      required: true

  - type: textarea
    id: doctor
    attributes:
      label: cleo doctor output (optional)
      description: Output of `cleo doctor`. Helpful for hook-related issues.
      render: text

  - type: textarea
    id: errors
    attributes:
      label: Hook error log excerpt (optional)
      description: Last 20 lines of `~/.config/cleo/hook-errors.log` if relevant.
      render: text
```

- [ ] **Step 9.3: Update CHANGELOG**

Under `### Added`:

```markdown
- `CONTRIBUTING.md` documenting build/test commands, commit format, and manual verification rituals.
- Bug-report issue template at `.github/ISSUE_TEMPLATE/bug_report.yml`.
```

- [ ] **Step 9.4: Commit**

```bash
git add CONTRIBUTING.md .github/ISSUE_TEMPLATE/bug_report.yml CHANGELOG.md
git commit -m "docs: add CONTRIBUTING.md and bug report issue template"
```

---

## Task 10: Cut the v0.2.0 release

**Goal:** Once Tasks 1-9 are merged into `main`, tag `v0.2.0` and let the release workflow publish it.

- [ ] **Step 10.1: Update CHANGELOG header**

Run `date -u +%Y-%m-%d` to get today's UTC date. In `CHANGELOG.md`, replace the `## [Unreleased]` heading with:

```markdown
## [Unreleased]

## [0.2.0] - 2026-MM-DD
```

Substitute `2026-MM-DD` with the date from the command above. Move every `### Added` / `### Changed` / `### Fixed` block that was under `[Unreleased]` to under `[0.2.0]`, leaving `[Unreleased]` empty above it.

- [ ] **Step 10.2: Update README banner**

Edit `README.md`. Replace the v0.1 alpha banner with v0.2:

```markdown
> **Status:** v0.2. Pre-1.0 — config and CLI surface may change between minor releases. Bug reports and feedback welcome.
```

- [ ] **Step 10.3: Bump version constant**

Edit `internal/cli/root.go`:

```go
var Version = "0.2.0"
```

- [ ] **Step 10.4: Verify build and tests**

Run: `go build -o /tmp/cleo-build ./cmd/cleo && go vet ./... && go test ./... -count=1 && /tmp/cleo-build --version`
Expected: PASS, prints `cleo version 0.2.0`.

- [ ] **Step 10.5: Commit**

```bash
git add CHANGELOG.md README.md internal/cli/root.go
git commit -m "chore(release): prep v0.2.0"
```

- [ ] **Step 10.6: Create and push the tag**

```bash
git tag -a v0.2.0 -m "cleo v0.2.0"
git push origin main
git push origin v0.2.0
```

The `release.yml` workflow will run goreleaser and publish the GitHub release plus the homebrew formula update.

- [ ] **Step 10.7: Smoke-test the published binary**

```bash
brew update
brew upgrade cleo
cleo --version
```

Expected: `cleo version 0.2.0`.

---

## Done

All tasks complete when:
- 9 PRs merged to `main`
- `v0.2.0` tag pushed
- GitHub release exists with checksums + tarballs
- `brew upgrade cleo` installs the new version
- README and CHANGELOG reflect v0.2.0
- `docs/superpowers/backlog.md` retains every deferred item

Items deferred to v0.3 (already in backlog): real opencode/pi hook plugins, `cleo logs` aggregator, structured JSON logging, `cleo trace` command, ANSI passthrough in pane preview, multi-session preview cache, "tmux pane alive but agent dead" detection, reconciler state-machine refactor, per-session sound mute, `multi_match_first` fallback reason, goreleaser brews → casks migration, Windows support, CI matrix, branch protection, feature-request issue template, README screencast.
