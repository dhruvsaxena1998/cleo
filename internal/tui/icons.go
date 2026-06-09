package tui

// IconSet is the single source of every decorative glyph the dashboard draws:
// session-state markers, the project tree's folder carets, and the small chrome
// icons in the topbar, footer, and panels. Three sets are shipped — "nerd"
// (default, Nerd Font glyphs), "unicode" (broadly-portable symbols), and
// "ascii" (last-resort plain text) — selected by the ui.icons config key and
// resolved once into Theme.Icons. Code never hard-codes a glyph; it reads it
// from the resolved set so a single config flip restyles the whole UI.
//
// Glyphs are stored as literal runes; the trailing comment on each line names
// the glyph (e.g. fa-folder-o) since Private-Use codepoints render blank in
// most editors.
//
// A field may be "" to mean "this set has no icon here"; withIcon drops the icon
// (and its trailing space) entirely in that case, so the unicode/ascii sets stay
// clean instead of rendering filler.
type IconSet struct {
	// Name identifies the set ("nerd"/"unicode"/"ascii"). It keeps IconSet a
	// comparable, all-scalar struct (so it can be == compared) while letting
	// code that needs to branch on the set — e.g. spinner — do so without a
	// slice field.
	Name string

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

// Nerd Font glyphs. Codepoints are in the BMP Private Use Area (drawn at a
// single cell, so lipgloss.Width agrees with the terminal) from the stable
// FontAwesome / Powerline ranges. The state circles and folders use the
// outline (…-o) variants rather than the filled ones, so the markers read as
// light and uncramped next to text instead of heavy ink-blobs.
var nerdIcons = IconSet{
	Name:      "nerd",
	Running:   "", // fa-circle (filled) — small dot for the topbar "live" pill
	Waiting:   "", // fa-question-circle — needs input, kept prominent
	Idle:      "", // fa-circle-o (outline)
	Spawning:  "", // fa-spinner (static; running/spawning animate via spinner())
	Completed: "", // fa-check-circle-o (outline)
	Error:     "", // fa-times-circle-o (outline)
	Dead:      "", // fa-minus-circle

	FolderClosed: "", // fa-folder-o (outline)
	FolderOpen:   "", // fa-folder-open-o (outline)

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
	Project:  "", // fa-folder-o (matches the outline tree folders)
}

// Unicode glyphs — the pre-overhaul look. Portable symbols only; chrome icons
// are left empty so a non-Nerd-Font terminal renders clean text, not filler.
var unicodeIcons = IconSet{
	Name:      "unicode",
	Running:   "●", // ● black circle
	Waiting:   "◑", // ◑ circle right half black
	Idle:      "○", // ○ white circle
	Spawning:  "◌", // ◌ dotted circle
	Completed: "✓", // ✓ check mark
	Error:     "✗", // ✗ ballot x
	Dead:      "·", // · middle dot

	FolderClosed: "▸", // ▸ small right-pointing triangle
	FolderOpen:   "▾", // ▾ small down-pointing triangle

	SoundOn: "♪", // ♪ eighth note
}

// ASCII glyphs — last resort for terminals that mangle everything above 0x7f.
var asciiIcons = IconSet{
	Name:      "ascii",
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

// spinner returns the animation frames shown for the "working" states
// (running, spawning) in place of the static marker. Braille reads well and is
// single-cell in both Nerd Font and plain Unicode terminals; the ascii set
// keeps a pure-ASCII spinner so an icons="ascii" terminal still animates.
func (ic IconSet) spinner() []string {
	if ic.Name == "ascii" {
		return []string{"|", "/", "-", "\\"}
	}
	return []string{
		"⠋", "⠙", "⠹", "⠸", "⠼",
		"⠴", "⠦", "⠧", "⠇", "⠏",
	}
}

// withIcon prefixes text with icon and two spaces of breathing room, but only
// when icon is non-empty — so a set that omits a glyph renders just the text
// with no leading gap. Two spaces (not one) keep the marker from reading as
// cramped against the text, and leave a full space even on terminals that draw
// a Nerd Font glyph slightly wider than one cell.
func withIcon(icon, text string) string {
	if icon == "" {
		return text
	}
	return icon + "  " + text
}
