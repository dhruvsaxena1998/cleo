# Help Panel Redesign

**Date:** 2026-05-14  
**Status:** approved  
**File:** `internal/tui/popup_help.go`

## Summary

Replace the current single-column keybindings popup with a wider two-column panel that also surfaces icon meanings, filter behavior, and config file pointers — all without scrolling or tabs.

## Current State

`popup_help.go` renders a 48-char-wide modal popup with four keybinding sections (Navigation, Session Actions, Global, tmux). It has no icon legend, no filter explanation, and no config reference.

## Design

### Layout

Single bordered panel, ~92 chars wide, with a vertical divider splitting left and right columns (~42 chars of content each).

```
┌──────────────────────────────────────────┬──────────────────────────────────────────┐
│ Help                                     │                      esc / q to close   │
├──────────────────────────────────────────┼──────────────────────────────────────────┤
│  [left column content]                   │  [right column content]                  │
└──────────────────────────────────────────┴──────────────────────────────────────────┘
```

Title bar spans the full width. "Help" on the left (accent color), "esc / q to close" on the right (overlay0 color). The center divider row uses `├───┬───┤` / `└───┴───┘` box-drawing chars.

### Left Column — Inputs

Sections rendered top to bottom with a blank line between each:

| Section | Keys |
|---------|------|
| Navigation | `↑/k` up · `↓/j` down · `space` expand/collapse |
| Session Actions | `↵` attach · `v` view pane · `n` new session · `r` rename · `K` kill · `P` prune finished · `D` remove project |
| Global | `/` filter · `m` mute/unmute · `?` help · `q` quit |
| tmux | `<detach_key>` detach — return to cleo (substituted dynamically from config) |

### Right Column — Reference

| Section | Content |
|---------|---------|
| Icon Legend | `◉` working (blue) · `⚠` needs input (gold) · `✓` completed (green) · `✗` failed (red) · `∙` idle (dimmed) · `○` stopped (dimmed) |
| Filter | "type to match project · session · agent" · "case-insensitive · esc to clear" |
| Config | Path `~/.config/cleo/config.toml` as section header, then key names: `defaults.detach_key` · `defaults.default_agent` · `ui.theme` · `ui.show_pane_preview` · `agents.<name>` |

### Column Height Alignment

The left column will always be taller than the right. After rendering both columns as row slices, pad the bottom of the right column with blank rows until heights match, then stitch row-by-row.

### Colors

Reuse the existing theme fields — no new theme fields needed:

| Element | Theme field |
|---------|-------------|
| Border chars | `Overlay1` |
| Section headers | `Overlay0` |
| Keys / icon glyphs | `Gold` |
| Descriptions | `Subtext0` |
| Title "Help" | `Accent` (bold) |
| Close hint | `Overlay0` |
| Filter description text | `Subtext0` (same as other descriptions) |
| Config key names | `Mauve` |
| Icon colors | Existing display-state colors (blue/gold/green/red/dimmed) |

## Implementation Approach

Row-by-row stitching using the existing `strings.Builder` pattern:

1. Pre-render left column rows as `[]string` (no outer border)
2. Pre-render right column rows as `[]string` (no outer border)
3. Pad the shorter slice with blank strings to equal length
4. Write top border: `┌─(left)─┬─(right)─┐`
5. Write title row: `│ Help ... │ ... esc/q │`
6. Write divider: `├─(left)─┼─(right)─┤`
7. For each row index: write `│ leftRow │ rightRow │`
8. Write bottom border: `└─(left)─┴─(right)─┘`

No new dependencies. No new files. All changes are within `popup_help.go`.

## Constraints

- Total popup width ~92 chars — fits comfortably in an 100-wide terminal; degrades gracefully (truncation) in narrower terminals via the existing `truncateWidth` helper
- Detach key substituted at render time from `HelpPopup.detachKey` (already done)
- No scrolling, no tabs — fixed height, all content visible at once
- `HelpPopup` struct signature unchanged; no changes to callers

## Out of Scope

- Dynamic/responsive column widths based on terminal size
- Scrollable help content
- Hyperlinks or clickable config file path
