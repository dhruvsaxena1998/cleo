package tui

// LayoutMode is the dashboard's responsive mode, derived purely from terminal size.
type LayoutMode int

const (
	// LayoutFull is the multi-panel desktop layout: sidebar + right column.
	LayoutFull LayoutMode = iota
	// LayoutCompact collapses to a single full-width sidebar for phone-width terminals.
	LayoutCompact
)

// compactWidthThreshold is the terminal width (in columns) below which the
// dashboard collapses to compact mode. Chosen for portrait phone-over-SSH
// widths of roughly 40–55. Intentionally a fixed constant — no config key yet.
const compactWidthThreshold = 60

// Layout describes how renderFrame should size each region for a given terminal.
// It centralizes the dimension math that was previously inlined in renderFrame
// and renderRightColumn so the decision is a pure, testable function of size.
type Layout struct {
	Mode     LayoutMode
	Width    int // normalized terminal width
	BodyH    int // height available between the topbar and footer
	SidebarW int
	MainW    int // right-column width; 0 in compact mode

	// Right-column panel heights (full mode only; all 0 in compact).
	MetaH    int // fixed Session metadata grid
	EventsH  int // events log strip
	PreviewH int // tmux preview (remainder)
}

// decideLayout maps a terminal size and the configured sidebar width to a Layout.
func decideLayout(width, height, sidebarWidth int) Layout {
	if width <= 0 {
		width = 120
	}
	if height <= 0 {
		height = 40
	}

	bodyH := height - 2 // topbar (1) + footer (1)
	if bodyH < 8 {
		bodyH = 8
	}

	// Below the threshold, collapse to a single full-width sidebar and omit the
	// right column entirely (events + tmux preview + metadata grid).
	if width < compactWidthThreshold {
		return Layout{
			Mode:     LayoutCompact,
			Width:    width,
			BodyH:    bodyH,
			SidebarW: width,
		}
	}

	sideW := sidebarWidth
	if sideW > width-40 {
		sideW = width - 40
	}
	if sideW < 10 {
		sideW = 10
	}
	mainW := width - sideW
	if mainW < 40 {
		mainW = 40
	}

	const metaH = 6 // border(2) + title(1) + sep(1) + labels(1) + values(1)
	eventsH := bodyH * 28 / 100
	if eventsH < 6 {
		eventsH = 6
	}
	previewH := bodyH - metaH - eventsH
	if previewH < 5 {
		previewH = 5
	}

	return Layout{
		Mode:     LayoutFull,
		Width:    width,
		BodyH:    bodyH,
		SidebarW: sideW,
		MainW:    mainW,
		MetaH:    metaH,
		EventsH:  eventsH,
		PreviewH: previewH,
	}
}
