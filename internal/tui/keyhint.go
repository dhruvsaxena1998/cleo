package tui

import (
	"strings"

	"github.com/charmbracelet/bubbles/key"
)

// keyGlyphs maps raw bubbletea key names to the compact glyphs the dashboard
// shows. It is the single source for prettifying keys in both the help popup
// and the footer hints — no other glyph table should exist.
var keyGlyphs = map[string]string{
	"up":    "↑",
	"down":  "↓",
	"left":  "←",
	"right": "→",
	"enter": "↵",
	" ":     "space",
}

// prettifyKey renders a raw key string as its display glyph, falling back to the
// key itself for anything without a glyph (single runes, ctrl+/alt+ combos).
func prettifyKey(k string) string {
	if g, ok := keyGlyphs[k]; ok {
		return g
	}
	return k
}

// keysLabel renders every key bound to an action, prettified and joined with
// "/" — the help popup's full-list label (e.g. "↑/k", "K/ctrl+k"). It returns
// "" for an action that resolved to zero keys under a hostile config.
func keysLabel(b key.Binding) string {
	keys := b.Keys()
	out := make([]string, len(keys))
	for i, k := range keys {
		out[i] = prettifyKey(k)
	}
	return strings.Join(out, "/")
}

// firstKeyGlyph renders the first key of an action — the footer's compact label.
// It returns "" for a zero-key action so callers never index keys[0] blind.
func firstKeyGlyph(b key.Binding) string {
	keys := b.Keys()
	if len(keys) == 0 {
		return ""
	}
	return prettifyKey(keys[0])
}
