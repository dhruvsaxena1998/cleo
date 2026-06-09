package tui

// IconSet is the single source of every decorative glyph the dashboard draws:
// session-state markers, the project tree's folder carets, and the small chrome
// icons in the topbar, footer, and panels. Three sets are shipped — "nerd"
// (default, Nerd Font glyphs), "unicode" (broadly-portable symbols), and
// "ascii" (last-resort plain text) — selected by the ui.icons config key and
// resolved once into Theme.Icons. Code never hard-codes a glyph; it reads it
// from the resolved set so a single config flip restyles the whole UI.
//
// A field may be "" to mean "this set has no icon here"; withIcon drops the icon
// (and its trailing space) entirely in that case, so the unicode/ascii sets stay
// clean instead of rendering filler.
type IconSet struct {
	// Session-state markers, indexed by the lifecycle states in styles.go.
	Running   string
	Waiting   string
	Idle      string
	Spawning  string
	Completed string
	Error     string
	Dead      string

	// Project tree.
	FolderClosed string
	FolderOpen   string

	// Chrome — topbar, footer, panels.
	Logo     string
	Branch   string
	Clock    string
	Memory   string
	SoundOn  string
	SoundOff string
	Session  string
	Events   string
	Tool     string
	Search   string
	Project  string
}

// Nerd Font glyphs, written as explicit codepoints so the source is unambiguous
// and editor-safe. Every codepoint is in the BMP Private Use Area and is drawn
// at a single cell, so lipgloss.Width agrees with the terminal and column
// alignment holds. They are drawn from the stable FontAwesome / Powerline ranges
// (U+E0xx, U+F0xx–U+F2xx) present in every Nerd Font build.
var nerdIcons = IconSet{
	Running:   "", // fa-circle (filled)
	Waiting:   "", // fa-question-circle — waiting for input
	Idle:      "", // fa-circle-o (hollow)
	Spawning:  "", // fa-spinner
	Completed: "", // fa-check-circle
	Error:     "", // fa-times-circle
	Dead:      "", // fa-minus-circle

	FolderClosed: "", // fa-folder
	FolderOpen:   "", // fa-folder-open

	Logo:     "", // fa-rocket
	Branch:   "", // powerline git branch
	Clock:    "", // fa-clock-o
	Memory:   "", // fa-microchip
	SoundOn:  "", // fa-volume-up
	SoundOff: "", // fa-volume-off
	Session:  "", // fa-terminal
	Events:   "", // fa-bolt
	Tool:     "", // fa-wrench
	Search:   "", // fa-search
	Project:  "", // fa-folder
}

// Unicode glyphs — the pre-overhaul look. Portable symbols only; chrome icons
// are left empty so a non-Nerd-Font terminal renders clean text, not filler.
var unicodeIcons = IconSet{
	Running:   "●",
	Waiting:   "◑",
	Idle:      "○",
	Spawning:  "◌",
	Completed: "✓",
	Error:     "✗",
	Dead:      "·",

	FolderClosed: "▸",
	FolderOpen:   "▾",

	SoundOn: "♪",
}

// ASCII glyphs — last resort for terminals that mangle everything above 0x7f.
var asciiIcons = IconSet{
	Running:   "*",
	Waiting:   "?",
	Idle:      "o",
	Spawning:  "~",
	Completed: "+",
	Error:     "x",
	Dead:      "-",

	FolderClosed: ">",
	FolderOpen:   "v",

	SoundOn: "*",
}

// iconSetNames is the selectable glyph sets in the order the settings editor
// cycles them — richest first. resolveIcons maps any of these (and unknown
// values) to a set.
var iconSetNames = []string{"nerd", "unicode", "ascii"}

// IconSetNames returns the glyph-set names the in-app settings editor offers,
// mirroring ThemeNames for the theme field.
func IconSetNames() []string {
	return append([]string(nil), iconSetNames...)
}

// resolveIcons maps the ui.icons config value to a set, defaulting to nerd for
// the empty or unknown value. The default lives here (not config) so an
// out-of-range value degrades to the shipped look instead of erroring.
func resolveIcons(name string) IconSet {
	switch name {
	case "unicode":
		return unicodeIcons
	case "ascii":
		return asciiIcons
	default:
		return nerdIcons
	}
}

// withIcon prefixes text with icon and a single space, but only when icon is
// non-empty — so a set that omits a glyph renders just the text with no
// leading gap.
func withIcon(icon, text string) string {
	if icon == "" {
		return text
	}
	return icon + " " + text
}
