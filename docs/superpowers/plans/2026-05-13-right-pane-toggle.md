# Right Pane Toggle Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the fixed events-above-preview right column layout with two toggle-able modes — Event log (timeline view) and Terminal preview — switched with `tab`.

**Architecture:** Add a `RightPaneMode` field to `Model`, wire a `Tab` keybinding that flips it, gate tmux polling on the active mode, and replace `renderRightColumn`'s three-panel stack with a tab-bar panel box that renders the correct mode body. Timeline logic (event pairing, Now card, history dots) lives in a new `timeline.go` file.

**Tech Stack:** Go, Bubble Tea (bubbletea), Lipgloss, `internal/events` (events.Entry + Log.Tail), `internal/state` (State constants)

---

## File Map

| File | Action | Responsibility |
|------|--------|---------------|
| `internal/tui/model.go` | Modify | Add `RightPaneMode` type + `rightPaneMode` field |
| `internal/tui/keymap.go` | Modify | Add `Tab` binding |
| `internal/tui/handle_key.go` | Modify | Add `tab` case + `toggleRightPane`; gate `autoCaptureCmd` |
| `internal/tui/update.go` | Modify | Gate `previewTickMsg` capture on `rightPaneMode` |
| `internal/tui/styles.go` | Modify | Add `TabPanelBox` method |
| `internal/tui/timeline.go` | Create | `pairedEvent`, `pairEvents`, `toolIcon`, progress bar, history rendering |
| `internal/tui/main_pane.go` | Modify | Rewrite `renderRightColumn`; add tab-bar panel builders |
| `internal/tui/tui_test.go` | Modify | New tests for all tasks |

---

## Task 1: RightPaneMode type + Tab keybinding

**Files:**
- Modify: `internal/tui/model.go`
- Modify: `internal/tui/keymap.go`
- Modify: `internal/tui/tui_test.go`

- [ ] **Step 1: Write the failing test**

Add to `internal/tui/tui_test.go`:

```go
func TestRightPaneModeDefaultsToEventLog(t *testing.T) {
	c := newTestCtx(t)
	m := New(c)
	if m.rightPaneMode != RightPaneModeEventLog {
		t.Errorf("expected RightPaneModeEventLog on New(), got %v", m.rightPaneMode)
	}
}
```

- [ ] **Step 2: Run the test to verify it fails**

```
go test ./internal/tui/... -run TestRightPaneModeDefaultsToEventLog -v
```

Expected: compile error — `RightPaneModeEventLog` undefined

- [ ] **Step 3: Add RightPaneMode type and field to model.go**

In `internal/tui/model.go`, add after the `Mode` type block (after line 56, `ModePopup`):

```go
type RightPaneMode int

const (
	RightPaneModeEventLog RightPaneMode = iota
	RightPaneModeTerminal
)
```

Add `rightPaneMode RightPaneMode` to the `Model` struct, after the `heapAlloc` field:

```go
type Model struct {
	ctx           *cli.Ctx
	theme         Theme
	projects      []projects.Project
	sessions      []state.Session
	cursor        cursor
	expanded      map[string]bool
	paneCache     map[string]string
	selected      string
	status        string
	filter        string
	mode          Mode
	popup         tea.Model
	help          help.Model
	width, height int
	err           error

	paneCaptureInFlight bool
	firstStateLoaded    bool
	heapAlloc           uint64
	rightPaneMode       RightPaneMode
}
```

- [ ] **Step 4: Add Tab binding to keymap.go**

Replace the `Keymap` struct and `DefaultKeymap` function in `internal/tui/keymap.go`:

```go
package tui

import "github.com/charmbracelet/bubbles/key"

type Keymap struct {
	Up, Down, Enter, New, View, Kill, Prune, Rename, Filter, Mute, Help, Quit, Esc, Space, Tab key.Binding
}

func DefaultKeymap() Keymap {
	return Keymap{
		Up:     key.NewBinding(key.WithKeys("up", "k"), key.WithHelp("↑/k", "up")),
		Down:   key.NewBinding(key.WithKeys("down", "j"), key.WithHelp("↓/j", "down")),
		Enter:  key.NewBinding(key.WithKeys("enter"), key.WithHelp("↵", "attach")),
		New:    key.NewBinding(key.WithKeys("n"), key.WithHelp("n", "new")),
		View:   key.NewBinding(key.WithKeys("v"), key.WithHelp("v", "view")),
		Kill:   key.NewBinding(key.WithKeys("K", "ctrl+k"), key.WithHelp("K", "kill")),
		Prune:  key.NewBinding(key.WithKeys("P"), key.WithHelp("P", "prune finished")),
		Rename: key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "rename")),
		Filter: key.NewBinding(key.WithKeys("/"), key.WithHelp("/", "filter")),
		Mute:   key.NewBinding(key.WithKeys("m"), key.WithHelp("m", "mute")),
		Help:   key.NewBinding(key.WithKeys("?"), key.WithHelp("?", "help")),
		Quit:   key.NewBinding(key.WithKeys("q"), key.WithHelp("q", "quit")),
		Esc:    key.NewBinding(key.WithKeys("esc")),
		Space:  key.NewBinding(key.WithKeys(" "), key.WithHelp("space", "expand / collapse")),
		Tab:    key.NewBinding(key.WithKeys("tab"), key.WithHelp("tab", "toggle pane")),
	}
}
```

- [ ] **Step 5: Run test to verify it passes**

```
go test ./internal/tui/... -run TestRightPaneModeDefaultsToEventLog -v
```

Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add internal/tui/model.go internal/tui/keymap.go internal/tui/tui_test.go
git commit -m "feat(tui): add RightPaneMode type and Tab keybinding scaffold"
```

---

## Task 2: Handle Tab key + gate polling in event log mode

**Files:**
- Modify: `internal/tui/handle_key.go`
- Modify: `internal/tui/update.go`
- Modify: `internal/tui/tui_test.go`

- [ ] **Step 1: Write failing tests**

Add to `internal/tui/tui_test.go`:

```go
func TestTabTogglesRightPaneMode(t *testing.T) {
	c := newTestCtx(t)
	c.Config.UI.ShowPanePreview = true
	m := New(c)

	if m.rightPaneMode != RightPaneModeEventLog {
		t.Fatalf("expected EventLog mode initially, got %v", m.rightPaneMode)
	}
	m2 := updateAsModel(m, tea.KeyMsg{Type: tea.KeyTab})
	if m2.rightPaneMode != RightPaneModeTerminal {
		t.Errorf("first tab: want Terminal, got %v", m2.rightPaneMode)
	}
	m3 := updateAsModel(m2, tea.KeyMsg{Type: tea.KeyTab})
	if m3.rightPaneMode != RightPaneModeEventLog {
		t.Errorf("second tab: want EventLog, got %v", m3.rightPaneMode)
	}
}

func TestTabNoOpWhenPreviewDisabled(t *testing.T) {
	c := newTestCtx(t)
	c.Config.UI.ShowPanePreview = false
	m := New(c)

	m2 := updateAsModel(m, tea.KeyMsg{Type: tea.KeyTab})
	if m2.rightPaneMode != RightPaneModeEventLog {
		t.Errorf("tab should not change mode when preview disabled, got %v", m2.rightPaneMode)
	}
}

func TestPreviewTickSkippedInEventLogMode(t *testing.T) {
	c := newTestCtx(t)
	c.Config.UI.ShowPanePreview = true
	c.Config.UI.PanePreviewInterval = 10 * time.Millisecond
	m := New(c)
	m.projects = []projects.Project{{ID: "p"}}
	m.sessions = []state.Session{{ID: "s1", State: state.Running, ProjectID: "p"}}
	m.cursor.projectIdx = 0
	m.cursor.agentIdx = 0
	m.expanded = map[string]bool{"p": true}
	// rightPaneMode defaults to RightPaneModeEventLog

	updated, cmd := m.Update(previewTickMsg{})
	if cmd == nil {
		t.Fatal("previewTickMsg must always re-arm the ticker")
	}
	out := runCmdAndCollect(t, cmd, 200*time.Millisecond)
	if !containsType(out, previewTickMsg{}) {
		t.Fatal("must re-arm with previewTickMsg even in EventLog mode")
	}
	if updated.(Model).paneCaptureInFlight {
		t.Error("must not dispatch a tmux capture in EventLog mode")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

```
go test ./internal/tui/... -run "TestTabToggles|TestTabNoOp|TestPreviewTickSkipped" -v
```

Expected: FAIL — `toggleRightPane` undefined

- [ ] **Step 3: Add toggleRightPane and gate autoCaptureCmd in handle_key.go**

Add the `tab` case to the switch in `handleKey` (before the closing brace, after the `Space` case):

```go
case key.Matches(msg, km.Tab):
    return m.toggleRightPane()
```

Add `toggleRightPane` method after `toggleExpand` in `internal/tui/handle_key.go`:

```go
func (m Model) toggleRightPane() (Model, tea.Cmd) {
	if !m.ctx.Config.UI.ShowPanePreview {
		return m, nil
	}
	if m.rightPaneMode == RightPaneModeEventLog {
		m.rightPaneMode = RightPaneModeTerminal
		return m, m.autoCaptureCmd()
	}
	m.rightPaneMode = RightPaneModeEventLog
	return m, nil
}
```

In `autoCaptureCmd`, add the mode gate after the `ShowPanePreview` check:

```go
func (m Model) autoCaptureCmd() tea.Cmd {
	if !m.ctx.Config.UI.ShowPanePreview {
		return nil
	}
	if m.rightPaneMode == RightPaneModeEventLog {
		return nil
	}
	sess, ok := m.sessionAtCursor()
	if !ok {
		return nil
	}
	if sess.State.IsFinished() {
		return nil
	}
	return capturePaneCmd(m.ctx, sess.ID, m.ctx.Config.UI.PanePreviewLines)
}
```

- [ ] **Step 4: Gate previewTickMsg capture in update.go**

In `internal/tui/update.go`, in the `previewTickMsg` case, add the mode check after the `ShowPanePreview` check:

```go
case previewTickMsg:
    next := previewTickCmd(m.ctx.Config.UI.PanePreviewInterval)
    if !m.ctx.Config.UI.ShowPanePreview {
        return m, next
    }
    if m.rightPaneMode == RightPaneModeEventLog {
        return m, next
    }
    sess, ok := m.selectedSession()
    if !ok || sess.State.IsFinished() || m.paneCaptureInFlight {
        return m, next
    }
    m.paneCaptureInFlight = true
    return m, tea.Batch(next, capturePaneCmd(m.ctx, sess.ID, m.ctx.Config.UI.PanePreviewLines))
```

- [ ] **Step 5: Run tests to verify they pass**

```
go test ./internal/tui/... -run "TestTabToggles|TestTabNoOp|TestPreviewTickSkipped" -v
```

Expected: PASS

- [ ] **Step 6: Verify existing tests still pass**

```
go test ./internal/tui/... -v
```

Expected: all tests pass including `TestPreviewTickAlwaysReArms` (the gating must preserve the re-arm path)

- [ ] **Step 7: Commit**

```bash
git add internal/tui/handle_key.go internal/tui/update.go internal/tui/tui_test.go
git commit -m "feat(tui): handle tab key to toggle right pane mode, gate tmux polling"
```

---

## Task 3: Add TabPanelBox style helper

**Files:**
- Modify: `internal/tui/styles.go`
- Modify: `internal/tui/tui_test.go`

- [ ] **Step 1: Write the failing test**

Add to `internal/tui/tui_test.go`:

```go
func TestTabPanelBoxRendersTabLabels(t *testing.T) {
	c := newTestCtx(t)
	m := New(c)

	out := m.theme.TabPanelBox("Event log [active]", "Terminal [tab]", []string{"line1", "line2"}, 80, 12)
	if !strings.Contains(out, "Event log [active]") {
		t.Errorf("expected active tab label in output, got:\n%s", out)
	}
	if !strings.Contains(out, "Terminal [tab]") {
		t.Errorf("expected inactive tab label in output, got:\n%s", out)
	}
	if !strings.Contains(out, "tab to switch") {
		t.Errorf("expected key hint in output, got:\n%s", out)
	}
	if !strings.Contains(out, "line1") {
		t.Errorf("expected body content in output, got:\n%s", out)
	}
	for _, line := range strings.Split(out, "\n") {
		if visualWidth(line) > 80 {
			t.Errorf("line wider than panel width: %q (%d cells)", line, visualWidth(line))
		}
	}
}
```

- [ ] **Step 2: Run the test to verify it fails**

```
go test ./internal/tui/... -run TestTabPanelBoxRendersTabLabels -v
```

Expected: compile error — `TabPanelBox` undefined

- [ ] **Step 3: Add TabPanelBox to styles.go**

Add the following method to `internal/tui/styles.go`, after the `PanelBox` method:

```go
// TabPanelBox renders a bordered panel whose header row contains two tab labels
// (active and inactive) and a right-aligned "tab to switch" hint.
func (t Theme) TabPanelBox(activeLabel, inactiveLabel string, body []string, w, h int) string {
	iw := w - 2
	if iw < 4 {
		iw = 4
	}
	cUsable := iw - 2

	bdr := lipgloss.NewStyle().Foreground(t.Surf1).Background(t.Base)
	activeSt := lipgloss.NewStyle().Foreground(t.Accent).Bold(true).Background(t.Base)
	inactiveSt := lipgloss.NewStyle().Foreground(t.Overlay0).Background(t.Base)
	hintSt := lipgloss.NewStyle().Foreground(t.Overlay0).Background(t.Base)
	innerSt := lipgloss.NewStyle().Background(t.Base).Width(iw)

	hbar := strings.Repeat("─", iw)

	activeR := activeSt.Render(activeLabel)
	inactiveR := inactiveSt.Render(inactiveLabel)
	hint := hintSt.Render("tab to switch")

	// Gap between inactive label and hint; minimum 1 space.
	gap := cUsable - lipgloss.Width(activeR) - 2 - lipgloss.Width(inactiveR) - lipgloss.Width(hint)
	if gap < 1 {
		gap = 1
	}
	tabRow := activeR + "  " + inactiveR + strings.Repeat(" ", gap) + hint

	contentH := h - 4
	if contentH < 0 {
		contentH = 0
	}
	lines := make([]string, contentH)
	for i := 0; i < len(body) && i < contentH; i++ {
		lines[i] = body[i]
	}

	var b strings.Builder
	b.WriteString(bdr.Render("┌"+hbar+"┐") + "\n")
	b.WriteString(bdr.Render("│") + innerSt.Render(" "+padRight(tabRow, cUsable)) + bdr.Render("│") + "\n")
	b.WriteString(bdr.Render("├"+hbar+"┤") + "\n")
	for _, line := range lines {
		padded := padRight(line, cUsable)
		if lipgloss.Width(padded) > cUsable {
			padded = truncateWidth(padded, cUsable)
		}
		b.WriteString(bdr.Render("│") + innerSt.Render(" "+padded) + bdr.Render("│") + "\n")
	}
	b.WriteString(bdr.Render("└" + hbar + "┘"))
	return b.String()
}
```

- [ ] **Step 4: Run test to verify it passes**

```
go test ./internal/tui/... -run TestTabPanelBoxRendersTabLabels -v
```

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/tui/styles.go internal/tui/tui_test.go
git commit -m "feat(tui): add TabPanelBox style helper for two-tab panel"
```

---

## Task 4: Implement timeline event log rendering

**Files:**
- Create: `internal/tui/timeline.go`
- Modify: `internal/tui/tui_test.go`

- [ ] **Step 1: Write failing tests**

Add to `internal/tui/tui_test.go`:

```go
func TestPairEventsMatchesPreAndPost(t *testing.T) {
	now := time.Now()
	entries := []events.Entry{
		{Type: "PreToolUse", Tool: "Edit", At: now, Detail: "src/main.go"},
		{Type: "PostToolUse", Tool: "Edit", At: now.Add(time.Second), DurationS: 1.0},
	}
	nowEntry, history := pairEvents(entries)
	if nowEntry != nil {
		t.Errorf("expected nil now card when all tools paired, got %v", nowEntry)
	}
	if len(history) != 1 {
		t.Fatalf("expected 1 history entry, got %d", len(history))
	}
	if history[0].Post == nil {
		t.Error("expected paired entry to have Post set")
	}
	if history[0].Pre.Tool != "Edit" {
		t.Errorf("expected Pre.Tool=Edit, got %q", history[0].Pre.Tool)
	}
}

func TestPairEventsUnmatchedPreBecomeNowCard(t *testing.T) {
	entries := []events.Entry{
		{Type: "PreToolUse", Tool: "Bash", At: time.Now(), Detail: "go test ./..."},
	}
	nowEntry, history := pairEvents(entries)
	if nowEntry == nil {
		t.Fatal("expected unmatched PreToolUse to become now card")
	}
	if nowEntry.Tool != "Bash" {
		t.Errorf("expected now.Tool=Bash, got %q", nowEntry.Tool)
	}
	if len(history) != 0 {
		t.Errorf("expected empty history, got %d entries", len(history))
	}
}

func TestPairEventsSessionStartInHistory(t *testing.T) {
	entries := []events.Entry{
		{Type: "SessionStart", At: time.Now()},
		{Type: "PreToolUse", Tool: "Read", At: time.Now(), Detail: "go.mod"},
		{Type: "PostToolUse", Tool: "Read", At: time.Now(), DurationS: 0.1},
	}
	nowEntry, history := pairEvents(entries)
	if nowEntry != nil {
		t.Errorf("expected nil now card, got %v", nowEntry)
	}
	if len(history) != 2 {
		t.Fatalf("expected 2 history entries (SessionStart + Read pair), got %d", len(history))
	}
	// History is reversed (most recent first); Read pair should be first
	if history[0].Pre.Tool != "Read" {
		t.Errorf("most recent entry should be Read, got %q", history[0].Pre.Type)
	}
}

func TestBuildTimelineBodyHidesNowCardWhenIdle(t *testing.T) {
	c := newTestCtx(t)
	m := New(c)
	sess := state.Session{ID: "s1", State: state.Idle, ProjectID: "p"}
	body := m.buildTimelineBody(76, 20, sess, true)
	for _, line := range body {
		if strings.Contains(line, "Now") {
			t.Errorf("Now card should be hidden for idle sessions, found: %q", line)
		}
	}
}

func TestBuildTimelineBodyShowsNowCardWhenRunning(t *testing.T) {
	c := newTestCtx(t)
	// Write a PreToolUse event so the timeline has something to show
	evLog := c.Events("s1")
	_ = evLog.Append(events.Entry{
		Type: "PreToolUse", Tool: "Edit", At: time.Now(), Detail: "src/api.go",
	})

	m := New(c)
	sess := state.Session{ID: "s1", State: state.Running, ProjectID: "p"}
	body := m.buildTimelineBody(76, 20, sess, true)

	found := false
	for _, line := range body {
		if strings.Contains(line, "Now") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected Now card for running session with in-flight tool, got body: %v", body)
	}
}

func TestBuildTimelineBodyWaitingShowsLastMessage(t *testing.T) {
	c := newTestCtx(t)
	m := New(c)
	sess := state.Session{
		ID: "s1", State: state.WaitingForInput, ProjectID: "p",
		LastMessage: "Allow file write to /etc/hosts?",
	}
	body := m.buildTimelineBody(76, 20, sess, true)

	found := false
	for _, line := range body {
		if strings.Contains(line, "Allow file write") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected last_message text in waiting state body, got: %v", body)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

```
go test ./internal/tui/... -run "TestPairEvents|TestBuildTimeline" -v
```

Expected: compile error — `pairEvents`, `buildTimelineBody` undefined

- [ ] **Step 3: Create internal/tui/timeline.go**

```go
package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"

	"github.com/dhruvsaxena1998/cleo/internal/events"
	"github.com/dhruvsaxena1998/cleo/internal/state"
)

// pairedEvent holds a PreToolUse event and its matching PostToolUse (nil if still in-flight).
// Non-tool events (SessionStart, Stop, etc.) are stored with Post=nil.
type pairedEvent struct {
	Pre  events.Entry
	Post *events.Entry
}

// pairEvents pairs PreToolUse + PostToolUse entries by tool name (FIFO per tool).
// Unmatched PreToolUse entries are candidates for the Now card; the most recent one
// is returned as nowEntry. Non-tool events go to history as-is.
// History is returned in reverse chronological order (most recent first).
func pairEvents(entries []events.Entry) (now *events.Entry, history []pairedEvent) {
	// pending maps tool name → queue of entry indices with unmatched PreToolUse
	pending := map[string][]int{}
	var pairs []pairedEvent

	for i, e := range entries {
		switch e.Type {
		case "PreToolUse":
			pending[e.Tool] = append(pending[e.Tool], i)
		case "PostToolUse":
			if q := pending[e.Tool]; len(q) > 0 {
				preIdx := q[0]
				pending[e.Tool] = q[1:]
				pairs = append(pairs, pairedEvent{Pre: entries[preIdx], Post: &entries[i]})
			}
		default:
			pairs = append(pairs, pairedEvent{Pre: e})
		}
	}

	// Find the most recent in-flight PreToolUse as the Now card
	var latest *events.Entry
	for _, idxs := range pending {
		for _, idx := range idxs {
			e := entries[idx]
			if latest == nil || e.At.After(latest.At) {
				cp := entries[idx]
				latest = &cp
			}
		}
	}
	now = latest

	// Reverse so most recent is first
	for i, j := 0, len(pairs)-1; i < j; i, j = i+1, j-1 {
		pairs[i], pairs[j] = pairs[j], pairs[i]
	}
	return now, pairs
}

// toolIcon returns a short Unicode glyph for a tool name.
func toolIcon(name string) string {
	switch name {
	case "Edit", "Write":
		return "✎"
	case "Bash":
		return "⚙"
	case "Read":
		return "◎"
	}
	return "▶"
}

// renderProgressBar returns an indeterminate progress bar string of the given width.
// The block position advances with elapsed time, cycling every 2 seconds.
func renderProgressBar(width int, startAt time.Time) string {
	if width <= 0 {
		return ""
	}
	blockW := width / 4
	if blockW < 1 {
		blockW = 1
	}
	elapsed := time.Since(startAt).Milliseconds()
	travel := int64(width - blockW)
	if travel < 1 {
		travel = 1
	}
	pos := int((elapsed % 2000) * travel / 2000)
	var b strings.Builder
	for i := 0; i < width; i++ {
		if i >= pos && i < pos+blockW {
			b.WriteString("▓")
		} else {
			b.WriteString("░")
		}
	}
	return b.String()
}

// buildTimelineBody returns a slice of styled lines for the event log timeline content area.
// w is the inner content width (panel width minus borders and padding).
// h is the max number of lines to return.
func (m Model) buildTimelineBody(w, h int, sess state.Session, has bool) []string {
	faint := lipgloss.NewStyle().Foreground(m.theme.Overlay0)

	if !has {
		return []string{faint.Render("select a session to view events")}
	}

	log := m.ctx.Events(sess.ID)
	entries, _ := log.Tail(200)

	nowEntry, history := pairEvents(entries)

	var lines []string

	switch sess.State {
	case state.Running:
		if nowEntry != nil {
			lines = append(lines, m.buildNowCardLines(w, nowEntry)...)
			lines = append(lines, "") // spacer
		}
	case state.WaitingForInput:
		lines = append(lines, m.buildWaitingCardLines(w, sess)...)
		lines = append(lines, "") // spacer
	}

	for _, pair := range history {
		if len(lines) >= h {
			break
		}
		lines = append(lines, m.renderHistoryEntry(pair, w)...)
	}

	if len(lines) > h {
		lines = lines[:h]
	}
	return lines
}

// buildNowCardLines renders the "Now" card for a running session with an in-flight tool.
func (m Model) buildNowCardLines(w int, entry *events.Entry) []string {
	elapsed := int(time.Since(entry.At).Seconds())

	headerSt := lipgloss.NewStyle().Foreground(m.theme.Blue).Bold(true)
	timeSt := lipgloss.NewStyle().Foreground(m.theme.Overlay0)
	toolSt := lipgloss.NewStyle().Foreground(m.theme.Text)

	elapsedStr := timeSt.Render(fmt.Sprintf("%ds", elapsed))
	headerText := headerSt.Render("⟳ Now")
	gap := w - lipgloss.Width(headerText) - lipgloss.Width(elapsedStr)
	if gap < 1 {
		gap = 1
	}
	header := headerText + strings.Repeat(" ", gap) + elapsedStr

	icon := toolIcon(entry.Tool)
	detail := entry.Detail
	if detail == "" {
		detail = entry.Tool
	}
	detailW := w - 2 - lipgloss.Width(icon) - 1
	if detailW < 4 {
		detailW = 4
	}
	toolLine := "  " + icon + " " + toolSt.Render(truncateWidth(detail, detailW))

	barW := w - 4
	if barW < 4 {
		barW = 4
	}
	progressLine := "  " + lipgloss.NewStyle().Foreground(m.theme.Blue).Render(renderProgressBar(barW, entry.At))

	return []string{header, toolLine, progressLine}
}

// buildWaitingCardLines renders the Now area for a session waiting for input.
func (m Model) buildWaitingCardLines(w int, sess state.Session) []string {
	headerSt := lipgloss.NewStyle().Foreground(m.theme.Yellow).Bold(true)
	msgSt := lipgloss.NewStyle().Foreground(m.theme.Text).Italic(true)

	header := headerSt.Render("⚠ Waiting for input")
	msg := sess.LastMessage
	if msg == "" {
		msg = "permission request"
	}
	return []string{header, "  " + msgSt.Render(truncateWidth(msg, w-2))}
}

// renderHistoryEntry renders one or two lines for a single history entry.
func (m Model) renderHistoryEntry(pair pairedEvent, w int) []string {
	e := pair.Pre

	// Non-tool events (SessionStart, Stop, etc.)
	switch e.Type {
	case "SessionStart", "session_start":
		st := lipgloss.NewStyle().Foreground(m.theme.Accent)
		detail := fmt.Sprintf("SessionStart · %s", sinceLabel(e.At))
		return []string{st.Render("○ ") + lipgloss.NewStyle().Foreground(m.theme.Overlay0).Render(truncateWidth(detail, w-2))}
	case "Stop", "stop", "SessionEnd", "session_end":
		st := lipgloss.NewStyle().Foreground(m.theme.Blue)
		return []string{st.Render("● ") + lipgloss.NewStyle().Foreground(m.theme.Overlay0).Render(truncateWidth(e.Type, w-2))}
	case "Notification", "notification":
		st := lipgloss.NewStyle().Foreground(m.theme.Gold)
		msg := e.Detail
		if msg == "" {
			msg = "Notification"
		}
		return []string{st.Render("◆ ") + st.Render(truncateWidth(msg, w-2))}
	}

	// Tool invocations
	dotColor := m.timelineDotColor(pair)
	dot := lipgloss.NewStyle().Foreground(dotColor).Render("●")

	toolName := e.Tool
	detail := e.Detail
	if detail == "" {
		detail = e.Tool
	}

	durStr := ""
	if pair.Post != nil && pair.Post.DurationS > 0 {
		durStr = fmt.Sprintf("%.1fs", pair.Post.DurationS)
	}

	const nameW = 10
	const durW = 6
	detailW := w - 2 - nameW - 2 - durW
	if detailW < 4 {
		detailW = 4
	}

	toolSt := lipgloss.NewStyle().Foreground(m.theme.Text)
	detailSt := lipgloss.NewStyle().Foreground(m.theme.Subtext0)
	timeSt := lipgloss.NewStyle().Foreground(m.theme.Overlay0)

	line := dot + " " +
		padRight(toolSt.Render(truncateWidth(toolName, nameW)), nameW) +
		"  " + padRight(detailSt.Render(truncateWidth(detail, detailW)), detailW) +
		"  " + timeSt.Render(padRight(durStr, durW))

	result := []string{line}

	// Result line for Bash: parse exit code from Extra
	if (toolName == "Bash" || toolName == "bash") && pair.Post != nil {
		if ec, ok := pair.Post.Extra["exit_code"]; ok {
			exitStr := fmt.Sprintf("→ exit %v", ec)
			resultSt := lipgloss.NewStyle().Foreground(dotColor)
			result = append(result, "  "+resultSt.Render(truncateWidth(exitStr, w-4)))
		}
	}

	return result
}

// timelineDotColor returns the dot color for a history entry based on tool and exit code.
func (m Model) timelineDotColor(pair pairedEvent) lipgloss.Color {
	if pair.Post == nil {
		return m.theme.Overlay0
	}
	switch pair.Pre.Tool {
	case "Bash", "bash":
		if ec, ok := pair.Post.Extra["exit_code"]; ok {
			if ecFloat, ok := ec.(float64); ok && ecFloat != 0 {
				return m.theme.Red
			}
		}
		return m.theme.Green
	case "Edit", "Write", "Read":
		return m.theme.Overlay0
	}
	return m.theme.Green
}
```

- [ ] **Step 4: Run tests to verify they pass**

```
go test ./internal/tui/... -run "TestPairEvents|TestBuildTimeline" -v
```

Expected: PASS

- [ ] **Step 5: Verify full test suite**

```
go test ./internal/tui/... -v
```

Expected: all tests pass

- [ ] **Step 6: Commit**

```bash
git add internal/tui/timeline.go internal/tui/tui_test.go
git commit -m "feat(tui): implement timeline event log — pairEvents, Now card, history dots"
```

---

## Task 5: Rewire renderRightColumn with tab-bar mode switching

**Files:**
- Modify: `internal/tui/main_pane.go`
- Modify: `internal/tui/tui_test.go`

- [ ] **Step 1: Write failing tests**

Add to `internal/tui/tui_test.go`:

```go
func TestRenderRightColumnEventLogMode(t *testing.T) {
	c := newTestCtx(t)
	c.Config.UI.ShowPanePreview = true
	m := New(c)
	m.rightPaneMode = RightPaneModeEventLog
	m.sessions = []state.Session{{ID: "s1", State: state.Running, ProjectID: "p"}}
	m.projects = []projects.Project{{ID: "p"}}
	m.expanded = map[string]bool{"p": true}
	m.cursor.projectIdx = 0
	m.cursor.agentIdx = 0

	out := m.renderRightColumn(80, 40)
	if !strings.Contains(out, "Event log [active]") {
		t.Errorf("expected event log tab active, got:\n%s", out)
	}
	if !strings.Contains(out, "Terminal [tab]") {
		t.Errorf("expected terminal tab inactive, got:\n%s", out)
	}
}

func TestRenderRightColumnTerminalMode(t *testing.T) {
	c := newTestCtx(t)
	c.Config.UI.ShowPanePreview = true
	m := New(c)
	m.rightPaneMode = RightPaneModeTerminal
	m.sessions = []state.Session{{ID: "s1", State: state.Running, ProjectID: "p"}}
	m.projects = []projects.Project{{ID: "p"}}
	m.expanded = map[string]bool{"p": true}
	m.cursor.projectIdx = 0
	m.cursor.agentIdx = 0

	out := m.renderRightColumn(80, 40)
	if !strings.Contains(out, "Terminal [active]") {
		t.Errorf("expected terminal tab active, got:\n%s", out)
	}
	if !strings.Contains(out, "Event log [tab]") {
		t.Errorf("expected event log tab inactive, got:\n%s", out)
	}
}

func TestRenderRightColumnNoTabBarWhenPreviewDisabled(t *testing.T) {
	c := newTestCtx(t)
	c.Config.UI.ShowPanePreview = false
	m := New(c)
	m.sessions = []state.Session{{ID: "s1", State: state.Running, ProjectID: "p"}}
	m.projects = []projects.Project{{ID: "p"}}
	m.expanded = map[string]bool{"p": true}
	m.cursor.projectIdx = 0
	m.cursor.agentIdx = 0

	out := m.renderRightColumn(80, 40)
	if strings.Contains(out, "tab to switch") {
		t.Errorf("tab bar should be hidden when ShowPanePreview=false, got:\n%s", out)
	}
}

func TestStoppedSessionTerminalModeShowsPlaceholder(t *testing.T) {
	c := newTestCtx(t)
	c.Config.UI.ShowPanePreview = true
	m := New(c)
	m.rightPaneMode = RightPaneModeTerminal
	m.sessions = []state.Session{{ID: "s1", State: state.Dead, ProjectID: "p"}}
	m.projects = []projects.Project{{ID: "p"}}
	m.expanded = map[string]bool{"p": true}
	m.cursor.projectIdx = 0
	m.cursor.agentIdx = 0

	out := m.renderRightColumn(80, 40)
	if !strings.Contains(out, "Session stopped") {
		t.Errorf("expected placeholder text for stopped session in Terminal mode, got:\n%s", out)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

```
go test ./internal/tui/... -run "TestRenderRightColumn|TestStoppedSession" -v
```

Expected: FAIL — output won't contain new tab labels yet

- [ ] **Step 3: Rewrite renderRightColumn and add tab-bar builders in main_pane.go**

Replace the existing `renderRightColumn` function (lines 16–33) in `internal/tui/main_pane.go` with:

```go
// renderRightColumn assembles the right column: session metadata strip on top,
// then a tabbed content panel below showing either the event log timeline or the
// tmux terminal preview, toggled with tab.
func (m Model) renderRightColumn(w, h int) string {
	const metaH = 6
	contentH := h - metaH - 1 // -1 for the newline separator between panels
	if contentH < 6 {
		contentH = 6
	}

	sess, hasSess := m.selectedSession()
	meta := m.renderMetaPanel(w, metaH, sess, hasSess)

	// When pane preview is disabled, show only the event log with no tab bar.
	if !m.ctx.Config.UI.ShowPanePreview {
		return meta + "\n" + m.renderTimelinePanel(w, contentH, sess, hasSess)
	}

	switch m.rightPaneMode {
	case RightPaneModeTerminal:
		return meta + "\n" + m.renderTerminalWithTabBar(w, contentH, sess, hasSess)
	default:
		return meta + "\n" + m.renderEventLogWithTabBar(w, contentH, sess, hasSess)
	}
}

// renderTimelinePanel renders a standalone event log panel (no tab bar).
// Used when show_pane_preview = false.
func (m Model) renderTimelinePanel(w, h int, sess state.Session, has bool) string {
	body := m.buildTimelineBody(w-4, h-4, sess, has)
	return m.theme.PanelBox("Event log", "", body, w, h)
}

// renderEventLogWithTabBar renders the event log timeline in a TabPanelBox.
func (m Model) renderEventLogWithTabBar(w, h int, sess state.Session, has bool) string {
	body := m.buildTimelineBody(w-4, h-4, sess, has)
	return m.theme.TabPanelBox("● Event log [active]", "○ Terminal [tab]", body, w, h)
}

// renderTerminalWithTabBar renders the tmux terminal preview in a TabPanelBox.
func (m Model) renderTerminalWithTabBar(w, h int, sess state.Session, has bool) string {
	body := m.buildTerminalBody(w-4, h-4, sess, has)
	return m.theme.TabPanelBox("○ Event log [tab]", "● Terminal [active]", body, w, h)
}

// buildTerminalBody returns the terminal preview content lines.
func (m Model) buildTerminalBody(w, h int, sess state.Session, has bool) []string {
	faint := lipgloss.NewStyle().Foreground(m.theme.Overlay0)
	dimmed := lipgloss.NewStyle().Foreground(m.theme.Subtext0)

	if !has {
		return []string{faint.Render("navigate to a session to view its terminal")}
	}
	if sess.State == state.Dead {
		return []string{faint.Render("○  Session stopped — tmux session is no longer available")}
	}

	pane := m.paneCache[sess.ID]
	switch {
	case pane == "":
		return []string{faint.Render("loading…  press v to refresh")}
	case strings.TrimSpace(pane) == "":
		return []string{faint.Render("agent hasn't rendered yet — press Enter to attach")}
	}

	allLines := strings.Split(pane, "\n")
	for len(allLines) > 1 && strings.TrimSpace(allLines[len(allLines)-1]) == "" {
		allLines = allLines[:len(allLines)-1]
	}

	start := len(allLines) - h
	if start < 0 {
		start = 0
	}
	shown := allLines[start:]
	result := make([]string, len(shown))
	for i, l := range shown {
		result[i] = dimmed.Render(truncateWidth(l, w))
	}
	return result
}
```

Note: the `state` package is already imported in `main_pane.go` (it's used in `renderPreviewPanel`).

- [ ] **Step 4: Verify imports in main_pane.go are correct**

The current imports in `main_pane.go` are:
```go
import (
    "fmt"
    "strings"
    "time"

    "github.com/charmbracelet/lipgloss"
    "github.com/dhruvsaxena1998/cleo/internal/state"
)
```

The new code uses `state.Dead` and `strings.TrimSpace` — both already imported. No new imports needed.

- [ ] **Step 5: Run tests to verify they pass**

```
go test ./internal/tui/... -run "TestRenderRightColumn|TestStoppedSession" -v
```

Expected: PASS

- [ ] **Step 6: Run full test suite**

```
go test ./internal/tui/... -v
```

Expected: all existing tests pass. In particular `TestPreviewLinesAreTruncatedToPanelWidth`, `TestPreviewWhitespaceShowsAttachHint`, and `TestPreviewEmptyShowsLoading` still call `renderPreviewPanel` directly — that function is untouched and must still pass.

- [ ] **Step 7: Manual smoke test**

Launch cleo in a terminal with at least one active session:

```bash
go run ./cmd/cleo
```

- Navigate to a session row.
- Verify the right column shows "● Event log [active]" and "○ Terminal [tab]" in the tab bar.
- Press `tab` — verify the tab bar flips to "○ Event log [tab]" and "● Terminal [active]", and terminal preview content appears.
- Press `tab` again — verify return to event log view.
- Verify that while in Event log mode, no `tmux capture-pane` subprocess appears in `ps aux | grep tmux`.
- With `show_pane_preview = false` in config, verify no tab bar appears.

- [ ] **Step 8: Commit**

```bash
git add internal/tui/main_pane.go internal/tui/tui_test.go
git commit -m "feat(tui): wire tab-bar toggle into renderRightColumn — event log / terminal modes"
```

---

## Self-Review Checklist

**Spec coverage:**

| Spec requirement | Task |
|---|---|
| `tab` toggles right pane between Event log and Terminal | Task 2 (key handler) |
| Tab bar always visible with active/inactive labels | Task 3 + 5 |
| Tab bar hidden when `show_pane_preview = false` | Task 5 |
| Default mode = Event log | Task 1 (zero value) |
| Now card shown for `running` state | Task 4 |
| Now card shows `last_message` for `waiting_for_input` | Task 4 |
| Now card hidden for `idle`, `completed`, `failed`, `stopped` | Task 4 |
| Dot colors: green Bash exit 0, red non-zero, dim file ops | Task 4 |
| Terminal polling suspended in Event log mode | Task 2 |
| Terminal polling resumes (immediate capture) on switch to Terminal | Task 2 |
| Stopped session Terminal mode shows placeholder text | Task 5 |

**No placeholders:** All code blocks contain complete, runnable Go code.

**Type consistency:** `pairedEvent` defined in Task 4 (`timeline.go`), used in Task 4 only. `RightPaneMode` defined in Task 1 (`model.go`), used in Tasks 2 and 5. `TabPanelBox` defined in Task 3 (`styles.go`), called in Task 5.
