package tui

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"
	zone "github.com/lrstanley/bubblezone"

	"github.com/dhruvsaxena1998/cleo/internal/projects"
	"github.com/dhruvsaxena1998/cleo/internal/state"
	"testing"
)

// waitForZone polls until the zone manager's background worker has recorded the
// given zone. zone.Scan (called by View) buffers zone bounds to a goroutine, so
// an immediate Get may miss them — this races under load (e.g. CI running the
// full suite). Production is unaffected: clicks arrive many frames after a scan.
func waitForZone(t *testing.T, id string) *zone.ZoneInfo {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for {
		if z := zone.Get(id); !z.IsZero() {
			return z
		}
		if time.Now().After(deadline) {
			t.Fatalf("zone %q was not registered by View within timeout", id)
		}
		time.Sleep(time.Millisecond)
	}
}

// treeFixture builds a model with one expanded project holding two running
// sessions, so the sidebar renders header + two session rows.
func treeFixture(t *testing.T) Model {
	t.Helper()
	c := newTestCtx(t)
	m := New(c)
	m.projects = []projects.Project{{ID: "myapp", Name: "myapp", Path: "/tmp/myapp"}}
	m.sessions = []state.Session{
		{ID: "cleo-myapp-claude-1", ProjectID: "myapp", Agent: "claude", Name: "one", State: state.Running, StartedAt: time.Now()},
		{ID: "cleo-myapp-claude-2", ProjectID: "myapp", Agent: "claude", Name: "two", State: state.Running, StartedAt: time.Now()},
	}
	m.expanded = map[string]bool{"myapp": true}
	m.width, m.height = 120, 40
	return m
}

// Wheel events are position-independent, so they exercise cursor movement
// without any rendered-coordinate dependency.
func TestMouseWheelMovesCursor(t *testing.T) {
	m := treeFixture(t)
	m.cursor.projectIdx = 0
	m.cursor.agentIdx = -1 // on the project header

	down := tea.MouseMsg{Action: tea.MouseActionPress, Button: tea.MouseButtonWheelDown}
	gotModel, _ := m.handleMouse(down)
	if got := gotModel.cursor.agentIdx; got != 0 {
		t.Fatalf("after wheel down agentIdx = %d, want 0 (moved onto first session)", got)
	}

	up := tea.MouseMsg{Action: tea.MouseActionPress, Button: tea.MouseButtonWheelUp}
	gotModel, _ = gotModel.handleMouse(up)
	if got := gotModel.cursor.agentIdx; got != -1 {
		t.Fatalf("after wheel up agentIdx = %d, want -1 (back on project header)", got)
	}
}

// Clicking a session row selects it. The click coordinate is taken from the
// zone's own recorded bounds (populated by View), so the test stays correct
// regardless of panel chrome or scroll-offset math.
func TestMouseClickSelectsSessionRow(t *testing.T) {
	m := treeFixture(t)
	m.cursor.projectIdx = 0
	m.cursor.agentIdx = -1

	_ = m.View() // marks rows and records their screen bounds

	z := waitForZone(t, sessZoneID(0, 1))
	click := tea.MouseMsg{X: z.StartX, Y: z.StartY, Action: tea.MouseActionPress, Button: tea.MouseButtonLeft}
	got, _ := m.handleMouse(click)
	if got.cursor.projectIdx != 0 || got.cursor.agentIdx != 1 {
		t.Fatalf("after click cursor = {%d,%d}, want {0,1}", got.cursor.projectIdx, got.cursor.agentIdx)
	}
}

// Clicking a project header selects it and toggles expansion.
func TestMouseClickTogglesProjectExpansion(t *testing.T) {
	m := treeFixture(t)
	m.cursor.projectIdx = 0
	m.cursor.agentIdx = -1
	_ = m.View()

	z := waitForZone(t, projZoneID(0))
	click := tea.MouseMsg{X: z.StartX, Y: z.StartY, Action: tea.MouseActionPress, Button: tea.MouseButtonLeft}
	got, _ := m.handleMouse(click)
	if got.expanded["myapp"] {
		t.Fatal("clicking expanded project header should collapse it")
	}
}
