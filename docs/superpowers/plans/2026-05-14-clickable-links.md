# Clickable Links in Attached Sessions Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Set `allow-passthrough on` on every cleo-spawned tmux pane so terminal emulators (Ghostty, WezTerm, modern iTerm2) can detect plain-text URLs and make them cmd+clickable.

**Architecture:** One additional tmux command is issued inside `(*Client).NewSession` immediately after the session is created. The call is best-effort — its error is silently discarded so older tmux versions (< 3.3a that don't support `allow-passthrough`) degrade gracefully with no visible change.

**Tech Stack:** Go, tmux, `os/exec`

---

### Task 1: Add `allow-passthrough on` to NewSession

**Files:**
- Modify: `internal/tmux/tmux.go:36-55`
- Modify: `internal/tmux/tmux_test.go` (add one test)

- [ ] **Step 1: Write the failing test**

The test file is `package tmux` (internal package), uses the `newTestClient(t)` helper already defined in the file, and accesses `c.cmd` directly. Add this test at the end of `internal/tmux/tmux_test.go`:

```go
func TestNewSession_SetsAllowPassthrough(t *testing.T) {
	c := newTestClient(t)
	name := "cleo-pt-test-1"
	if err := c.NewSession(NewSessionOpts{Name: name, Cwd: "/tmp", Cmd: "sleep 60"}); err != nil {
		t.Fatal(err)
	}
	out, err := c.cmd("show-options", "-pt", name, "allow-passthrough").Output()
	if err != nil {
		t.Skipf("tmux version does not support allow-passthrough: %v", err)
	}
	if !strings.Contains(string(out), "allow-passthrough on") {
		t.Errorf("expected allow-passthrough on, got: %q", string(out))
	}
}
```

- [ ] **Step 2: Run the test to verify it fails**

```bash
go test ./internal/tmux/... -run TestNewSession_SetsAllowPassthrough -v
```

Expected: FAIL with output like `expected allow-passthrough on, got: ""`

- [ ] **Step 3: Implement the fix**

In `internal/tmux/tmux.go`, add the `set-option` call after the `new-session` command succeeds. The full updated `NewSession` function:

```go
func (c *Client) NewSession(o NewSessionOpts) error {
	if o.Name == "" {
		return errors.New("tmux: empty session name")
	}
	args := []string{"new-session", "-d", "-s", o.Name}
	if o.Cwd != "" {
		args = append(args, "-c", o.Cwd)
	}
	for k, v := range o.Env {
		args = append(args, "-e", fmt.Sprintf("%s=%s", k, v))
	}
	if o.Cmd != "" {
		args = append(args, o.Cmd)
	}
	out, err := c.cmd(args...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("tmux new-session: %w (%s)", err, strings.TrimSpace(string(out)))
	}
	_ = c.cmd("set-option", "-pt", o.Name, "allow-passthrough", "on").Run()
	return nil
}
```

No new imports are needed.

- [ ] **Step 4: Run the failing test again to verify it passes**

```bash
go test ./internal/tmux/... -run TestNewSession_SetsAllowPassthrough -v
```

Expected: PASS (or SKIP if tmux < 3.3a is installed)

- [ ] **Step 5: Run the full test suite to check for regressions**

```bash
go test ./...
```

Expected: all tests pass

- [ ] **Step 6: Commit**

```bash
git add internal/tmux/tmux.go internal/tmux/tmux_test.go
git commit -m "fix(tmux): set allow-passthrough on new sessions for clickable URLs (#37)"
```
