package tui

import "testing"

// shape is a terse constructor for treeShape: each pair is {sessions, expanded}.
func shape(rows ...projectRowShape) treeShape { return treeShape{rows: rows} }

func exp(sessions int) projectRowShape { return projectRowShape{expanded: true, sessions: sessions} }
func col(sessions int) projectRowShape { return projectRowShape{expanded: false, sessions: sessions} }

func TestCursorNavigation(t *testing.T) {
	// p0 expanded(2), p1 collapsed(3), p2 expanded(0), p3 expanded(1)
	tree := shape(exp(2), col(3), exp(0), exp(1))

	cases := []struct {
		name  string
		start cursor
		move  string
		want  cursor
	}{
		// Down walks into expanded sessions, then to the next header.
		{"down into sessions", cursor{0, projectRow}, "down", cursor{0, 0}},
		{"down within sessions", cursor{0, 0}, "down", cursor{0, 1}},
		{"down past last session to next header", cursor{0, 1}, "down", cursor{1, projectRow}},
		// A collapsed project's sessions are skipped entirely.
		{"down skips collapsed sessions", cursor{1, projectRow}, "down", cursor{2, projectRow}},
		// An expanded-but-empty project has no session rows to enter.
		{"down skips empty expanded project", cursor{2, projectRow}, "down", cursor{3, projectRow}},
		{"down into last project's session", cursor{3, projectRow}, "down", cursor{3, 0}},
		{"down stops at bottom", cursor{3, 0}, "down", cursor{3, 0}},

		// Up is the inverse.
		{"up within sessions to header", cursor{0, 0}, "up", cursor{0, projectRow}},
		{"up stops at top", cursor{0, projectRow}, "up", cursor{0, projectRow}},
		{"up first session to header", cursor{0, 1}, "up", cursor{0, 0}},
		// Up into a collapsed previous project lands on its header, not a session.
		{"up into collapsed project lands on header", cursor{2, projectRow}, "up", cursor{1, projectRow}},
		// Up into an expanded previous project lands on its last session.
		{"up into expanded project lands on last session", cursor{1, projectRow}, "up", cursor{0, 1}},
		// Up into an expanded-but-empty previous project lands on its header.
		{"up into empty expanded project lands on header", cursor{3, projectRow}, "up", cursor{2, projectRow}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var got cursor
			if c.move == "up" {
				got = c.start.up(tree)
			} else {
				got = c.start.down(tree)
			}
			if got != c.want {
				t.Errorf("%v.%s = %v, want %v", c.start, c.move, got, c.want)
			}
		})
	}
}

func TestCursorClamp(t *testing.T) {
	cases := []struct {
		name  string
		tree  treeShape
		start cursor
		want  cursor
	}{
		{"empty tree resolves to first header", shape(), cursor{5, 9}, cursor{0, projectRow}},
		{"project index past end clamps to last", shape(exp(1), exp(1)), cursor{9, projectRow}, cursor{1, projectRow}},
		{"negative project index clamps to first", shape(exp(1)), cursor{-3, projectRow}, cursor{0, projectRow}},
		{"agent index past session count clamps", shape(exp(2)), cursor{0, 9}, cursor{0, 1}},
		{"collapsed project drops agent to header", shape(col(5)), cursor{0, 3}, cursor{0, projectRow}},
		{"expanded empty project drops agent to header", shape(exp(0)), cursor{0, 4}, cursor{0, projectRow}},
		{"valid cursor is unchanged", shape(exp(3)), cursor{0, 2}, cursor{0, 2}},
		{"agent index below header clamps up", shape(exp(2)), cursor{0, -4}, cursor{0, projectRow}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := c.start.clamp(c.tree); got != c.want {
				t.Errorf("%v.clamp(%v) = %v, want %v", c.start, c.tree.rows, got, c.want)
			}
		})
	}
}

func TestCursorFlatRow(t *testing.T) {
	// rows: p0 hdr(0), s0(1), s1(2), p1 hdr(3) [collapsed, sessions hidden],
	// p2 hdr(4), s0(5)
	tree := shape(exp(2), col(3), exp(1))
	cases := []struct {
		name string
		cur  cursor
		want int
	}{
		{"first header", cursor{0, projectRow}, 0},
		{"first session", cursor{0, 0}, 1},
		{"second session", cursor{0, 1}, 2},
		{"collapsed project header skips its sessions", cursor{1, projectRow}, 3},
		{"header after collapsed project", cursor{2, projectRow}, 4},
		{"session after collapsed project", cursor{2, 0}, 5},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := c.cur.flatRow(tree); got != c.want {
				t.Errorf("%v.flatRow = %d, want %d", c.cur, got, c.want)
			}
		})
	}
}
