# Issue #24: Fix `up/k` Cursor Navigation Jump Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make `up/k` navigation in the TUI project/session list symmetric with `down/j` by landing on the previous project's last session when moving up from a project header row.

**Architecture:** Single function patch in `cursorUp()` — after decrementing `projectIdx`, check if the new project is expanded and has sessions; if so, set `agentIdx` to the last session index. Paired with a direct unit test that exercises the full up/down navigation sequence without a teatest harness.

**Tech Stack:** Go, Bubble Tea (`github.com/charmbracelet/bubbletea`)

---

## File Map

- Modify: `internal/tui/handle_key.go` — patch `cursorUp()` only (~4 lines added)
- Modify: `internal/tui/tui_test.go` — add `TestCursorUpDownNavigation`

---

### Task 1: Write the failing test

**Files:**
- Modify: `internal/tui/tui_test.go`

- [ ] **Step 1: Add the test to `tui_test.go`**

Append this function to the end of `internal/tui/tui_test.go`:

```go
// TestCursorUpDownNavigation locks in symmetric up/down navigation across
// project headers and session rows (issue #24). Uses direct method calls —
// no teatest harness required.
func TestCursorUpDownNavigation(t *testing.T) {
	c := newTestCtx(t)
	m := New(c)
	m.projects = []projects.Project{{ID: "p1"}, {ID: "p2"}}
	now := time.Now()
	m.sessions = []state.Session{
		{ID: "s1a", ProjectID: "p1", StartedAt: now},
		{ID: "s1b", ProjectID: "p1", StartedAt: now.Add(time.Second)},
		{ID: "s2a", ProjectID: "p2", StartedAt: now},
		{ID: "s2b", ProjectID: "p2", StartedAt: now.Add(time.Second)},
	}
	m.expanded = map[string]bool{"p1": true, "p2": true}
	// Start on p2 header row.
	m.cursor.projectIdx = 1
	m.cursor.agentIdx = -1

	type pos struct{ proj, agent int }
	steps := []struct {
		dir  string
		want pos
	}{
		{"up", pos{0, 1}},  // p1 last session
		{"up", pos{0, 0}},  // p1 first session
		{"up", pos{0, -1}}, // p1 header
		{"up", pos{0, -1}}, // no movement — already at top
		// reverse: navigate back down
		{"down", pos{0, 0}},  // p1 first session
		{"down", pos{0, 1}},  // p1 last session
		{"down", pos{1, -1}}, // p2 header
		{"down", pos{1, 0}},  // p2 first session
		{"down", pos{1, 1}},  // p2 last session
		{"down", pos{1, 1}},  // no movement — already at bottom
	}

	for i, s := range steps {
		var cmd tea.Cmd
		if s.dir == "up" {
			m, cmd = m.cursorUp()
		} else {
			m, cmd = m.cursorDown()
		}
		_ = cmd
		if m.cursor.projectIdx != s.want.proj || m.cursor.agentIdx != s.want.agent {
			t.Errorf("step %d (%s): want {proj=%d agent=%d}, got {proj=%d agent=%d}",
				i+1, s.dir, s.want.proj, s.want.agent, m.cursor.projectIdx, m.cursor.agentIdx)
		}
	}
}
```

- [ ] **Step 2: Run the test to confirm it fails**

```bash
go test ./internal/tui/... -run TestCursorUpDownNavigation -v
```

Expected: `FAIL` — step 1 (`up`) reports `want {proj=0 agent=1}, got {proj=0 agent=-1}` because `cursorUp` currently lands on the project header instead of the last session.

---

### Task 2: Implement the fix

**Files:**
- Modify: `internal/tui/handle_key.go` — `cursorUp()` function

- [ ] **Step 1: Replace the project-boundary branch in `cursorUp`**

In `internal/tui/handle_key.go`, find `cursorUp` (currently lines 127–140):

```go
func (m Model) cursorUp() (Model, tea.Cmd) {
	m.status = ""
	if m.cursor.agentIdx >= 0 {
		m.cursor.agentIdx--
		if m.cursor.agentIdx < 0 {
			m.cursor.agentIdx = -1
		}
		return m, m.autoCaptureCmd()
	}
	if m.cursor.projectIdx > 0 {
		m.cursor.projectIdx--
	}
	return m, m.autoCaptureCmd()
}
```

Replace it with:

```go
func (m Model) cursorUp() (Model, tea.Cmd) {
	m.status = ""
	if m.cursor.agentIdx >= 0 {
		m.cursor.agentIdx--
		if m.cursor.agentIdx < 0 {
			m.cursor.agentIdx = -1
		}
		return m, m.autoCaptureCmd()
	}
	if m.cursor.projectIdx > 0 {
		m.cursor.projectIdx--
		prevPID := m.visibleProjectIDs()[m.cursor.projectIdx]
		if m.expanded[prevPID] {
			if ss := m.sessionsFor(prevPID); len(ss) > 0 {
				m.cursor.agentIdx = len(ss) - 1
			}
		}
	}
	return m, m.autoCaptureCmd()
}
```

- [ ] **Step 2: Run the navigation test to confirm it passes**

```bash
go test ./internal/tui/... -run TestCursorUpDownNavigation -v
```

Expected: `PASS`

- [ ] **Step 3: Run the full test suite to confirm no regressions**

```bash
go test ./...
```

Expected: all tests pass, no failures.

- [ ] **Step 4: Commit**

```bash
git add internal/tui/handle_key.go internal/tui/tui_test.go
git commit -m "fix(tui): cursorUp lands on previous project's last session when expanded"
```
