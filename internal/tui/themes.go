package tui

import (
	"sort"

	"github.com/charmbracelet/lipgloss"
)

// Theme holds all semantic colors for the TUI. Add a new built-in theme by:
// 1. Declaring a new var block below
// 2. Adding one entry to registry
type Theme struct {
	Name     string
	Base     lipgloss.Color
	Mantle   lipgloss.Color
	Crust    lipgloss.Color
	Surf0    lipgloss.Color
	Surf1    lipgloss.Color
	Surf2    lipgloss.Color
	Overlay0 lipgloss.Color
	Overlay1 lipgloss.Color
	Text     lipgloss.Color
	Subtext1 lipgloss.Color
	Subtext0 lipgloss.Color
	Accent   lipgloss.Color
	Gold     lipgloss.Color
	Green    lipgloss.Color
	Red      lipgloss.Color
	Peach    lipgloss.Color
	Mauve    lipgloss.Color
	Blue     lipgloss.Color
	Yellow   lipgloss.Color

	// Icons is the resolved glyph set (ui.icons). It rides on the Theme because
	// the Theme is the render context already threaded through every styling
	// method, so attaching the glyph set here makes those methods icon-aware
	// without widening their signatures. Resolve defaults it to nerd; the model
	// overrides it from config in New/applyTheme.
	Icons IconSet
}

var catppuccinMocha = Theme{
	Name:     "catppuccin-mocha",
	Base:     "#1e1e2e",
	Mantle:   "#181825",
	Crust:    "#11111b",
	Surf0:    "#313244",
	Surf1:    "#45475a",
	Surf2:    "#585b70",
	Overlay0: "#6c7086",
	Overlay1: "#7f849c",
	Text:     "#cdd6f4",
	Subtext1: "#bac2de",
	Subtext0: "#a6adc8",
	Accent:   "#89b4fa",
	Gold:     "#f9e2af",
	Green:    "#a6e3a1",
	Red:      "#f38ba8",
	Peach:    "#fab387",
	Mauve:    "#cba6f7",
	Blue:     "#89b4fa",
	Yellow:   "#f9e2af",
}

var gruvboxDark = Theme{
	Name:     "gruvbox-dark",
	Base:     "#282828",
	Mantle:   "#1d2021",
	Crust:    "#141617",
	Surf0:    "#3c3836",
	Surf1:    "#504945",
	Surf2:    "#665c54",
	Overlay0: "#7c6f64",
	Overlay1: "#928374",
	Text:     "#ebdbb2",
	Subtext1: "#d5c4a1",
	Subtext0: "#bdae93",
	Accent:   "#458588",
	Gold:     "#d79921",
	Green:    "#98971a",
	Red:      "#cc241d",
	Peach:    "#d65d0e",
	Mauve:    "#b16286",
	Blue:     "#458588",
	Yellow:   "#d79921",
}

var onedark = Theme{
	Name:     "onedark",
	Base:     "#282c34",
	Mantle:   "#21252b",
	Crust:    "#181a1f",
	Surf0:    "#2c313a",
	Surf1:    "#3e4452",
	Surf2:    "#4b5263",
	Overlay0: "#636d83",
	Overlay1: "#7f848e",
	Text:     "#abb2bf",
	Subtext1: "#9da5b4",
	Subtext0: "#848da1",
	Accent:   "#61afef",
	Gold:     "#e5c07b",
	Green:    "#98c379",
	Red:      "#e06c75",
	Peach:    "#d19a66",
	Mauve:    "#c678dd",
	Blue:     "#61afef",
	Yellow:   "#e5c07b",
}

var voidTheme = Theme{
	Name:     "void",
	Base:     "#000000",
	Mantle:   "#0a0a0a",
	Crust:    "#050505",
	Surf0:    "#111111",
	Surf1:    "#1a1a1a",
	Surf2:    "#333333",
	Overlay0: "#555555",
	Overlay1: "#888888",
	Text:     "#ffffff",
	Subtext1: "#e0e0e0",
	Subtext0: "#aaaaaa",
	Accent:   "#0070f3",
	Gold:     "#f5a623",
	Green:    "#50e3c2",
	Red:      "#ff4d4d",
	Peach:    "#f5a623",
	Mauve:    "#7928ca",
	Blue:     "#0070f3",
	Yellow:   "#f5a623",
}

var synthwave = Theme{
	Name:     "synthwave",
	Base:     "#262335",
	Mantle:   "#241b2f",
	Crust:    "#171520",
	Surf0:    "#34294f",
	Surf1:    "#3d3060",
	Surf2:    "#4a3870",
	Overlay0: "#444251",
	Overlay1: "#848bbd",
	Text:     "#ffffff",
	Subtext1: "#b6b1b1",
	Subtext0: "#9d8bca",
	Accent:   "#ff7edb",
	Gold:     "#fede5d",
	Green:    "#72f1b8",
	Red:      "#fe4450",
	Peach:    "#ff8b39",
	Mauve:    "#9d8bca",
	Blue:     "#36f9f6",
	Yellow:   "#f3e70f",
}

var registry = map[string]Theme{
	"catppuccin-mocha": catppuccinMocha,
	"gruvbox-dark":     gruvboxDark,
	"onedark":          onedark,
	"void":             voidTheme,
	"synthwave":        synthwave,
}

// Resolve returns the named theme, falling back to catppuccin-mocha. The
// returned Theme carries the default (nerd) icon set so a Theme used without
// the model — direct Resolve callers, tests — still has non-empty glyphs; the
// model overrides Icons from ui.icons.
func Resolve(name string) Theme {
	t := catppuccinMocha
	if found, ok := registry[name]; ok {
		t = found
	}
	t.Icons = nerdIcons
	return t
}

// ThemeNames returns the registry's theme names in sorted order. It is the
// source the in-app settings editor cycles through when picking a theme.
func ThemeNames() []string {
	names := make([]string, 0, len(registry))
	for name := range registry {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
