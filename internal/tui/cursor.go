package tui

// projectRow is the agentIdx sentinel meaning "the project header itself is
// selected" rather than a session under it.
const projectRow = -1

// cursor is a position in the sidebar tree: projectIdx indexes the visible
// projects in display order, and agentIdx is either projectRow (the header) or
// the index of a session under that project when it is expanded.
//
// The cursor owns every positional rule — navigation, clamping, and flattening
// to a render row — so call sites no longer re-derive bounds from
// visibleProjectIDs()/sessionsFor(). Those rules depend only on the tree's
// shape (which projects are visible, whether each is expanded, how many
// sessions it holds), captured by treeShape; the cursor never needs session
// contents, which keeps it a pure, independently testable module.
type cursor struct {
	projectIdx int
	agentIdx   int
}

// treeShape is the flattened sidebar at one instant: the visible projects in
// display order, each with its expanded flag and session count. It is the only
// input the cursor needs to move, clamp, and flatten.
type treeShape struct {
	rows []projectRowShape
}

type projectRowShape struct {
	expanded bool
	sessions int
}

func (t treeShape) len() int { return len(t.rows) }

func (c cursor) onProject(pi int) bool { return c.projectIdx == pi && c.agentIdx == projectRow }

func (c cursor) onAgent(pi, ai int) bool { return c.projectIdx == pi && c.agentIdx == ai }

// up moves one visible row up: within a project's sessions, from the first
// session to its header, or from a header to the previous project — landing on
// that project's last session when it is expanded and non-empty. Movement stops
// at the top row.
func (c cursor) up(t treeShape) cursor {
	if c.agentIdx >= 0 {
		c.agentIdx--
		return c
	}
	if c.projectIdx > 0 {
		c.projectIdx--
		if prev := t.rows[c.projectIdx]; prev.expanded && prev.sessions > 0 {
			c.agentIdx = prev.sessions - 1
		}
	}
	return c
}

// down is the inverse of up: into a project's sessions when expanded, then on to
// the next project's header. Movement stops at the bottom row.
func (c cursor) down(t treeShape) cursor {
	if c.projectIdx < 0 || c.projectIdx >= t.len() {
		return c
	}
	if node := t.rows[c.projectIdx]; node.expanded && c.agentIdx+1 < node.sessions {
		c.agentIdx++
		return c
	}
	if c.projectIdx+1 < t.len() {
		c.projectIdx++
		c.agentIdx = projectRow
	}
	return c
}

// clamp returns the nearest valid cursor for the given shape: projectIdx into
// range, agentIdx collapsed to the header when the project is collapsed and
// otherwise bounded by the session count. An empty tree resolves to the header
// of a would-be first project.
func (c cursor) clamp(t treeShape) cursor {
	if t.len() == 0 {
		return cursor{projectIdx: 0, agentIdx: projectRow}
	}
	if c.projectIdx < 0 {
		c.projectIdx = 0
	}
	if c.projectIdx >= t.len() {
		c.projectIdx = t.len() - 1
	}
	node := t.rows[c.projectIdx]
	if !node.expanded {
		c.agentIdx = projectRow
		return c
	}
	if c.agentIdx >= node.sessions {
		c.agentIdx = node.sessions - 1
	}
	if c.agentIdx < projectRow {
		c.agentIdx = projectRow
	}
	return c
}

// flatRow is the 0-based index of the cursor among all rendered rows — one per
// project header plus one per session under an expanded project — used to
// position the viewport scroll.
func (c cursor) flatRow(t treeShape) int {
	row := 0
	for pi, node := range t.rows {
		if pi == c.projectIdx {
			if c.agentIdx < 0 {
				return row
			}
			return row + 1 + c.agentIdx
		}
		row++ // project header
		if node.expanded {
			row += node.sessions
		}
	}
	return 0
}
