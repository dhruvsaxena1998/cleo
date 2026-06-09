package tui

import (
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/lipgloss"

	"github.com/dhruvsaxena1998/cleo/internal/projects"
	"github.com/dhruvsaxena1998/cleo/internal/state"
)

// renderFixtureModel builds a populated dashboard model at the given size and
// icon set. It assigns the unexported view fields directly (rather than driving
// the async load) so the renderer has a representative, deterministic tree.
func renderFixtureModel(t *testing.T, icons string, w, h int) Model {
	t.Helper()
	c := newTestCtx(t)
	c.Config.UI.Icons = icons
	m := New(c)
	m.width, m.height = w, h
	m.projects = []projects.Project{{ID: "cleo"}, {ID: "website-frontend"}}
	m.expanded = map[string]bool{"cleo": true, "website-frontend": true}
	now := time.Now()
	m.sessions = []state.Session{
		{ID: "cleo-cleo-codex-payment", ProjectID: "cleo", Agent: "codex", Name: "payment-webhook-race", State: state.WaitingForInput, StartedAt: now.Add(-42 * time.Minute), LastEventAt: now.Add(-18 * time.Second), ToolCount: 31},
		{ID: "cleo-cleo-claude-redesign", ProjectID: "cleo", Agent: "claude", Name: "redesign-terminal-ui", State: state.Running, StartedAt: now.Add(-18 * time.Minute), LastEventAt: now.Add(-2 * time.Minute)},
		{ID: "cleo-cleo-opencode-recon", ProjectID: "cleo", Agent: "opencode", Name: "tmux-reconcile-tests", State: state.Idle, StartedAt: now.Add(-7 * time.Minute)},
		{ID: "cleo-web-claude-pricing", ProjectID: "website-frontend", Agent: "claude", Name: "fix-pricing-copy", State: state.WaitingForInput, StartedAt: now.Add(-1 * time.Hour)},
	}
	m.cursor = m.cursor.clamp(m.treeShape())
	// A mid-spinner frame, so the width invariant is checked while a running
	// session renders an animated marker rather than the static frame 0.
	m.animFrame = 7
	return m
}

// TestRenderFrameLinesFillWidth guards the dashboard's column alignment: every
// rendered line must be exactly the frame width, with no row overflowing. This
// is the invariant most at risk from the icon overhaul — a glyph whose
// lipgloss.Width disagrees with the layout maths (a wrong codepoint, or padding
// slipped into a key chip) shows up here as a line that is one or more cells too
// wide or too narrow. It runs across all three icon sets and two terminal sizes.
func TestRenderFrameLinesFillWidth(t *testing.T) {
	for _, icons := range []string{"nerd", "unicode", "ascii"} {
		for _, size := range []struct{ w, h int }{{120, 40}, {80, 24}} {
			m := renderFixtureModel(t, icons, size.w, size.h)
			for i, l := range strings.Split(renderFrame(m), "\n") {
				if got := lipgloss.Width(l); got != size.w {
					t.Errorf("icons=%s %dx%d: line %d width=%d, want %d: %q",
						icons, size.w, size.h, i, got, size.w, l)
				}
			}
		}
	}
}
