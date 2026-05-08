# Theming Design

**Date:** 2026-05-08
**Status:** Approved

## Overview

Make the TUI color scheme configurable via `cleo.toml`. Ship 5 built-in themes: catppuccin-mocha, gruvbox-dark, onedark, void, synthwave. Users pick a theme by name; adding or removing themes in future requires touching one file.

## Scope

- Built-in themes only (no user-defined custom themes in v1)
- No `cleo themes` CLI command — valid names documented in the generated config comment
- Theme applies to the TUI only; CLI output (`cleo ls`, etc.) is unaffected

## Config

Add `theme` to the `[ui]` section of `cleo.toml`:

```toml
[ui]
# theme: catppuccin-mocha | gruvbox-dark | onedark | void | synthwave
theme = "catppuccin-mocha"
```

Go struct change in `internal/config/config.go`:

```go
type UI struct {
    // existing fields unchanged
    Theme string `toml:"theme"`
}
```

Default in `Defaults_()`: `Theme: "catppuccin-mocha"`. `mergeDefaults` currently replaces the whole `UI` block only when `SidebarWidth == 0`, so it won't backfill `Theme` on existing configs. Add an explicit guard:

```go
if c.UI.Theme == "" {
    c.UI.Theme = d.UI.Theme
}
```

## Architecture

### `internal/tui/themes.go` (new file)

Single source of truth for all theme data. Contains:

1. **`Theme` struct** — semantic color fields used throughout the TUI:
   - Surfaces: `Base`, `Mantle`, `Crust`, `Surf0`, `Surf1`, `Surf2`
   - Text: `Text`, `Subtext1`, `Subtext0`
   - Overlays: `Overlay0`, `Overlay1`
   - Accents: `Accent`, `Gold`, `Green`, `Red`, `Peach`, `Mauve`, `Blue`, `Yellow`

2. **5 theme vars** — one `var` block per theme with all hex values filled in

3. **`registry map[string]Theme`** — maps config name strings to theme vars. Adding a theme = one map entry + one var block. Removing = delete both.

4. **`Resolve(name string) Theme`** — looks up registry, falls back to `catppuccinMocha`

### Theme palette reference

| Theme | Background | Foreground | Accent |
|-------|-----------|------------|--------|
| catppuccin-mocha | `#1e1e2e` | `#cdd6f4` | `#89b4fa` |
| gruvbox-dark | `#282828` | `#ebdbb2` | `#458588` |
| onedark | `#282c34` | `#abb2bf` | `#61afef` |
| void | `#000000` | `#ffffff` | `#0070f3` (vercel blue) |
| synthwave | `#241734` | `#f2f2f2` | `#ff2d78` (neon pink) |

### `internal/tui/model.go`

Add `theme Theme` field to `Model`. Set in `New()`:

```go
func New(ctx *cli.Ctx) Model {
    return Model{
        ctx:   ctx,
        theme: Resolve(ctx.Config.UI.Theme),
        // ...existing fields
    }
}
```

### `internal/tui/styles.go`

- **Remove** all color `var` blocks (`clrBase`, `clrText`, semantic aliases, etc.)
- **Remove** all `lipgloss.Style` package-level vars (`styleSelected`, `styleDimmed`, etc.)
- **Keep** pure layout helpers: `padRight`, `truncateWidth`, `sectionDivider`, `stateGlyph`
- `stateColor`, `styledGlyph`, `styledStateText` → `Theme` methods (they directly map state strings to theme colors)
- `agentBadge`, `pill`, `keyHint`, `panelBox` → free functions taking `Theme` as first argument (they are rendering utilities, not color lookups)

### Render files (`view.go`, `sidebar.go`, `main_pane.go`, `popup_*.go`)

All references to removed color vars and style vars are replaced with inline `lipgloss.NewStyle()` calls referencing `m.theme.XXX`. Since all render functions are already methods on `Model`, `m.theme` is always in scope.

## Error handling

Unknown theme name in config → `Resolve()` silently falls back to catppuccin-mocha. No startup error, no warning (keeps UX simple; user can see the valid names in the config comment).

## Testing

No new test files needed. Existing `tui_test.go` constructs a `Model` — it will automatically use catppuccin-mocha (the fallback). Theme correctness is visual, not unit-testable.

## Files changed

| File | Change |
|------|--------|
| `internal/config/config.go` | Add `Theme string` to `UI` struct |
| `internal/config/defaults.go` | Add `Theme: "catppuccin-mocha"` to `UI` default |
| `internal/tui/themes.go` | **New** — `Theme` struct, 5 theme vars, registry, `Resolve()` |
| `internal/tui/styles.go` | Remove color vars and style vars; keep layout helpers |
| `internal/tui/model.go` | Add `theme Theme` field; set in `New()` |
| `internal/tui/view.go` | Replace color/style var refs with `m.theme.XXX` |
| `internal/tui/sidebar.go` | Same |
| `internal/tui/main_pane.go` | Same |
| `internal/tui/popup_*.go` | Same (4 popup files) |
