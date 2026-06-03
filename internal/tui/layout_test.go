package tui

import (
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/dhruvsaxena1998/cleo/internal/projects"
	"github.com/dhruvsaxena1998/cleo/internal/state"
)

// mkCompactTestModel parks the cursor on a single running session inside an
// expanded project, so View() renders both the sidebar and (in full mode) the
// right column with its three panels.
func mkCompactTestModel(t *testing.T) Model {
	t.Helper()
	m := New(newTestCtx(t))
	m.projects = []projects.Project{{ID: "myapp"}}
	m.sessions = []state.Session{{ID: "s1", ProjectID: "myapp", Agent: "claude", State: state.Running, StartedAt: time.Now()}}
	m.expanded = map[string]bool{"myapp": true}
	m.cursor.projectIdx = 0
	m.cursor.agentIdx = 0
	return m
}

// TestViewCompactOmitsRightColumn drives the responsive dashboard at two widths:
// a phone-width terminal collapses to the sidebar alone (no events / preview /
// metadata panels), while a desktop-width terminal still renders the full stack.
func TestViewCompactOmitsRightColumn(t *testing.T) {
	m := mkCompactTestModel(t)

	m.width, m.height = 120, 40
	full := ansi.Strip(m.View())
	if !strings.Contains(full, "Projects / Sessions") {
		t.Errorf("full layout should render the sidebar, got:\n%s", full)
	}
	for _, panel := range []string{"Events", "Terminal Preview"} {
		if !strings.Contains(full, panel) {
			t.Errorf("full layout should render the %q panel, got:\n%s", panel, full)
		}
	}

	m.width, m.height = 45, 40
	compact := ansi.Strip(m.View())
	if !strings.Contains(compact, "Projects / Sessions") {
		t.Errorf("compact layout should still render the sidebar, got:\n%s", compact)
	}
	for _, panel := range []string{"Events", "Terminal Preview"} {
		if strings.Contains(compact, panel) {
			t.Errorf("compact layout should omit the %q panel, got:\n%s", panel, compact)
		}
	}
}

// TestViewCompactNoOverflow pins width-aware truncation: at a phone-width
// terminal, long Project/Session names and the topbar must truncate so that no
// rendered line exceeds the terminal width and nothing wraps onto extra rows
// (a wrapped line would push the frame past its fixed height).
func TestViewCompactNoOverflow(t *testing.T) {
	const long = "a-very-long-name-that-would-overflow-a-narrow-terminal"
	const w, h = 45, 40

	cursors := []struct {
		name     string
		agentIdx int
	}{
		{"project header selected", -1},
		{"session row selected", 0},
	}
	for _, cur := range cursors {
		t.Run(cur.name, func(t *testing.T) {
			m := New(newTestCtx(t))
			m.projects = []projects.Project{{ID: long}}
			m.sessions = []state.Session{{ID: "s1", ProjectID: long, Agent: "claude", Name: long, State: state.Running, StartedAt: time.Now()}}
			m.expanded = map[string]bool{long: true}
			m.cursor.projectIdx = 0
			m.cursor.agentIdx = cur.agentIdx
			m.width, m.height = w, h

			lines := strings.Split(m.View(), "\n")
			for i, line := range lines {
				if lipgloss.Width(line) > w {
					t.Errorf("line %d exceeds width %d: %q (%d cells)", i, w, ansi.Strip(line), lipgloss.Width(line))
				}
			}
			if len(lines) != h {
				t.Errorf("compact frame should be exactly %d lines (no wrap), got %d", h, len(lines))
			}
		})
	}
}

// TestSidebarSelectedRowTruncates locks in story 7 for the highlighted row:
// the selected Project header and the selected Session row must truncate to the
// inner width instead of wrapping (the non-selected rows already truncate).
func TestSidebarSelectedRowTruncates(t *testing.T) {
	const long = "a-very-long-name-that-overflows-a-narrow-sidebar"
	const innerW = 30

	cases := []struct {
		name     string
		agentIdx int
	}{
		{"selected project header", -1},
		{"selected session row", 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m := New(newTestCtx(t))
			m.projects = []projects.Project{{ID: long}}
			m.sessions = []state.Session{{ID: "s1", ProjectID: long, Agent: "claude", Name: long, State: state.Running, StartedAt: time.Now()}}
			m.expanded = map[string]bool{long: true}
			m.cursor.projectIdx = 0
			m.cursor.agentIdx = tc.agentIdx

			lines := strings.Split(m.renderTreeContent(innerW), "\n")
			// One project header + one session row = exactly two rows, no wrap.
			if len(lines) != 2 {
				t.Errorf("tree should be 2 rows (no wrap), got %d:\n%s", len(lines), ansi.Strip(strings.Join(lines, "\n")))
			}
			for i, line := range lines {
				if lipgloss.Width(line) > innerW {
					t.Errorf("row %d exceeds innerW %d: %q (%d cells)", i, innerW, ansi.Strip(line), lipgloss.Width(line))
				}
			}
		})
	}
}

// decideLayout is pure, so its whole behaviour is a table: given the terminal
// width/height and the configured sidebar width, assert the resulting layout
// mode and region sizes. No bubbletea, no Model. Prior art: internal/hooks/decide_test.go.
func TestDecideLayout(t *testing.T) {
	tests := []struct {
		name                string
		width, height, side int

		wantMode     LayoutMode
		wantWidth    int
		wantBodyH    int
		wantSidebarW int
		wantMainW    int
		wantMetaH    int
		wantEventsH  int
		wantPreviewH int
	}{
		{
			name:  "wide terminal is full with config sidebar and remaining main",
			width: 120, height: 40, side: 30,
			wantMode: LayoutFull, wantWidth: 120, wantBodyH: 38, wantSidebarW: 30, wantMainW: 90,
			// bodyH=38 → meta fixed 6, events 38*28/100=10, preview remainder 22
			wantMetaH: 6, wantEventsH: 10, wantPreviewH: 22,
		},
		{
			name:  "oversize sidebar clamps to width minus 40, main floors at 40",
			width: 120, height: 40, side: 200,
			wantMode: LayoutFull, wantWidth: 120, wantBodyH: 38, wantSidebarW: 80, wantMainW: 40,
			wantMetaH: 6, wantEventsH: 10, wantPreviewH: 22,
		},
		{
			name:  "undersize sidebar clamps up to the minimum of 10",
			width: 120, height: 40, side: 2,
			wantMode: LayoutFull, wantWidth: 120, wantBodyH: 38, wantSidebarW: 10, wantMainW: 110,
			wantMetaH: 6, wantEventsH: 10, wantPreviewH: 22,
		},
		{
			name:  "narrow terminal is compact: sidebar full width, no right column",
			width: 50, height: 40, side: 30,
			wantMode: LayoutCompact, wantWidth: 50, wantBodyH: 38, wantSidebarW: 50, wantMainW: 0,
			wantMetaH: 0, wantEventsH: 0, wantPreviewH: 0,
		},
		{
			name:  "exactly at the threshold is still full",
			width: 60, height: 40, side: 30,
			wantMode: LayoutFull, wantWidth: 60, wantBodyH: 38, wantSidebarW: 20, wantMainW: 40,
			wantMetaH: 6, wantEventsH: 10, wantPreviewH: 22,
		},
		{
			name:  "one column below the threshold flips to compact",
			width: 59, height: 40, side: 30,
			wantMode: LayoutCompact, wantWidth: 59, wantBodyH: 38, wantSidebarW: 59, wantMainW: 0,
			wantMetaH: 0, wantEventsH: 0, wantPreviewH: 0,
		},
		{
			name:  "zero size falls back to 120x40 full",
			width: 0, height: 0, side: 30,
			wantMode: LayoutFull, wantWidth: 120, wantBodyH: 38, wantSidebarW: 30, wantMainW: 90,
			wantMetaH: 6, wantEventsH: 10, wantPreviewH: 22,
		},
		{
			name:  "negative size falls back to 120x40 full",
			width: -5, height: -5, side: 30,
			wantMode: LayoutFull, wantWidth: 120, wantBodyH: 38, wantSidebarW: 30, wantMainW: 90,
			wantMetaH: 6, wantEventsH: 10, wantPreviewH: 22,
		},
		{
			name:  "tiny height floors body to 8 in compact",
			width: 20, height: 3, side: 30,
			wantMode: LayoutCompact, wantWidth: 20, wantBodyH: 8, wantSidebarW: 20, wantMainW: 0,
			wantMetaH: 0, wantEventsH: 0, wantPreviewH: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := decideLayout(tt.width, tt.height, tt.side)

			if got.Mode != tt.wantMode {
				t.Errorf("Mode = %v, want %v", got.Mode, tt.wantMode)
			}
			if got.Width != tt.wantWidth {
				t.Errorf("Width = %d, want %d", got.Width, tt.wantWidth)
			}
			if got.BodyH != tt.wantBodyH {
				t.Errorf("BodyH = %d, want %d", got.BodyH, tt.wantBodyH)
			}
			if got.SidebarW != tt.wantSidebarW {
				t.Errorf("SidebarW = %d, want %d", got.SidebarW, tt.wantSidebarW)
			}
			if got.MainW != tt.wantMainW {
				t.Errorf("MainW = %d, want %d", got.MainW, tt.wantMainW)
			}
			if got.MetaH != tt.wantMetaH {
				t.Errorf("MetaH = %d, want %d", got.MetaH, tt.wantMetaH)
			}
			if got.EventsH != tt.wantEventsH {
				t.Errorf("EventsH = %d, want %d", got.EventsH, tt.wantEventsH)
			}
			if got.PreviewH != tt.wantPreviewH {
				t.Errorf("PreviewH = %d, want %d", got.PreviewH, tt.wantPreviewH)
			}
		})
	}
}
