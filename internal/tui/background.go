package tui

import (
	"strconv"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// paintBackground gives every currently unpainted printable cell an explicit
// background. It is intentionally order-dependent: component surfaces paint
// first, then broader surfaces fill only cells whose background was reset.
// This keeps transparent terminal profiles from showing through without
// replacing intentional backgrounds such as Mantle or Surf0.
func paintBackground(rendered string, bg lipgloss.Color) string {
	const sentinel = "\x00"
	styledSentinel := lipgloss.NewStyle().Background(bg).Render(sentinel)
	backgroundSGR, _, ok := strings.Cut(styledSentinel, sentinel)
	if !ok || backgroundSGR == "" {
		return rendered
	}

	var out strings.Builder
	out.Grow(len(rendered) + len(backgroundSGR))
	backgroundSet := false
	for i := 0; i < len(rendered); {
		if rendered[i] == '\x1b' && i+1 < len(rendered) && rendered[i+1] == '[' {
			if end := csiSequenceEnd(rendered, i); end >= 0 {
				if rendered[end] == 'm' {
					backgroundSet = backgroundAfterSGR(rendered[i+2:end], backgroundSet)
				}
				out.WriteString(rendered[i : end+1])
				i = end + 1
				continue
			}
		}
		if rendered[i] != '\n' && !backgroundSet {
			out.WriteString(backgroundSGR)
			backgroundSet = true
		}
		out.WriteByte(rendered[i])
		i++
	}
	return out.String()
}

// csiSequenceEnd returns the index of the first CSI final byte. Besides SGR
// (...m), rendered frames contain Bubble Zone's private ...z markers; stopping
// at the first final byte keeps those zero-width markers intact.
func csiSequenceEnd(s string, start int) int {
	for i := start + 2; i < len(s); i++ {
		if s[i] >= 0x40 && s[i] <= 0x7e {
			return i
		}
	}
	return -1
}

// backgroundAfterSGR folds one SGR parameter list into the prior background
// state. Both semicolon and ITU-T T.416 colon-form extended colours are
// accepted; foreground colour payloads are skipped so RGB values cannot be
// mistaken for background commands.
func backgroundAfterSGR(params string, wasSet bool) bool {
	parts := strings.FieldsFunc(params, func(r rune) bool { return r == ';' || r == ':' })
	if len(parts) == 0 { // ESC[m is the short form of ESC[0m.
		return false
	}
	for i := 0; i < len(parts); i++ {
		p, err := strconv.Atoi(parts[i])
		if err != nil {
			continue
		}
		switch {
		case p == 0 || p == 49:
			wasSet = false
		case p >= 40 && p <= 47 || p >= 100 && p <= 107:
			wasSet = true
		case p == 38 || p == 48:
			wasSet = wasSet || p == 48
			if i+1 < len(parts) {
				mode, _ := strconv.Atoi(parts[i+1])
				if mode == 5 {
					i += 2
				} else if mode == 2 {
					i += 4
				}
			}
		}
	}
	return wasSet
}
