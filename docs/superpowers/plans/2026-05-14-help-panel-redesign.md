# Help Panel Redesign Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the single-column keybindings popup with a wider two-column panel that adds icon legend, filter description, and config pointers.

**Architecture:** Rewrite `HelpPopup.View()` in `popup_help.go` using a row-by-row stitching approach: collect left-column rows and right-column rows as `[]string` slices, pad the shorter slice to match heights, then write each row as `│ leftRow │ rightRow │` inside a single outer border. The `HelpPopup` struct and its constructor stay unchanged.

**Tech Stack:** Go, charmbracelet/lipgloss (already a dependency), `strings.Builder` (same pattern as current code).

---

## File Map

| Action | File | What changes |
|--------|------|--------------|
| Modify | `internal/tui/popup_help.go` | Rewrite `View()` — new two-column layout. Struct, constructor, `Init()`, `Update()` untouched. |
| Modify | `internal/tui/tui_test.go` | Add `TestHelpPopupView` covering width, sections, icons, detach key. |

---

## Task 1: Write the failing test

**Files:**
- Modify: `internal/tui/tui_test.go`

- [ ] **Step 1: Add `TestHelpPopupView` to `internal/tui/tui_test.go`**

Append this test to the end of `tui_test.go` (before the final closing brace is not needed — just add it at the bottom of the file):

```go
// TestHelpPopupView locks in the two-column help panel layout.
// It checks width, required sections, icons, and detach key substitution.
func TestHelpPopupView(t *testing.T) {
	theme := catppuccinMocha
	popup := NewHelpPopup(theme, "C-b d")
	out := popup.View()
	lines := strings.Split(out, "\n")

	// Every rendered line must fit within 91 terminal cells.
	const maxW = 91
	for i, line := range lines {
		if w := lipgloss.Width(line); w > maxW {
			t.Errorf("line %d too wide: %d > %d: %q", i, w, maxW, line)
		}
	}

	// All required section headers must appear.
	for _, want := range []string{
		"Navigation", "Session Actions", "Global", "tmux",
		"Icon Legend", "Filter", "Config",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing section %q in help output", want)
		}
	}

	// All six status icons must appear.
	for _, icon := range []string{"◉", "⚠", "✓", "✗", "∙", "○"} {
		if !strings.Contains(out, icon) {
			t.Errorf("missing icon %q in help output", icon)
		}
	}

	// Detach key should be formatted and present.
	if !strings.Contains(out, "ctrl+b d") {
		t.Errorf("detach key 'ctrl+b d' not found in help output")
	}

	// Config path must appear.
	if !strings.Contains(out, "~/.config/cleo/config.toml") {
		t.Errorf("config path not found in help output")
	}

	// Filter description must appear.
	if !strings.Contains(out, "project · session · agent") {
		t.Errorf("filter description not found in help output")
	}
}
```

- [ ] **Step 2: Run the test to confirm it fails**

```bash
cd /path/to/cleo && go test ./internal/tui/ -run TestHelpPopupView -v
```

Expected: FAIL — the current single-column `View()` is only 48 chars wide (lines will be narrower than 91 and content like "Icon Legend" will be missing).

---

## Task 2: Rewrite `popup_help.go` View()

**Files:**
- Modify: `internal/tui/popup_help.go`

- [ ] **Step 1: Replace the entire `View()` method with the two-column implementation**

Replace the `View()` method body (lines 38–114 of `popup_help.go`). Keep everything above it (`package`, imports, `HelpPopup` struct, `HelpClosed`, `NewHelpPopup`, `formatTmuxKey`, `Init()`, `Update()`) exactly as-is.

New `View()`:

```go
func (p HelpPopup) View() string {
	const colW = 42
	bdr := lipgloss.NewStyle().Foreground(p.theme.Overlay1)

	hbar := strings.Repeat("─", colW+2)
	topBar := "┌" + hbar + "┬" + hbar + "┐"
	midBar := "├" + hbar + "┼" + hbar + "┤"
	botBar := "└" + hbar + "┴" + hbar + "┘"

	sectionSt := lipgloss.NewStyle().Foreground(p.theme.Overlay0)
	keySt := lipgloss.NewStyle().Foreground(p.theme.Gold).Bold(true)
	descSt := lipgloss.NewStyle().Foreground(p.theme.Subtext0)
	mauveSt := lipgloss.NewStyle().Foreground(p.theme.Mauve)

	// ── left column: inputs ──────────────────────────────────────────────────

	type krow struct{ key, desc string }
	type section struct {
		title string
		rows  []krow
	}
	leftSections := []section{
		{"Navigation", []krow{
			{"↑ / k", "up"},
			{"↓ / j", "down"},
			{"space", "expand / collapse"},
		}},
		{"Session Actions", []krow{
			{"↵", "attach"},
			{"v", "view pane"},
			{"n", "new session"},
			{"r", "rename"},
			{"K", "kill session"},
			{"P", "prune finished"},
			{"D", "remove project"},
		}},
		{"Global", []krow{
			{"/", "filter"},
			{"m", "mute / unmute"},
			{"?", "help"},
			{"q", "quit"},
		}},
		{"tmux", []krow{
			{p.detachKey, "detach — return to cleo"},
		}},
	}

	var left []string
	for si, sec := range leftSections {
		if si > 0 {
			left = append(left, "")
		}
		left = append(left, "")
		left = append(left, sectionSt.Render(sec.title))
		for _, r := range sec.rows {
			left = append(left, fmt.Sprintf("  %s  %s", keySt.Render(r.key), descSt.Render(r.desc)))
		}
		left = append(left, "")
	}

	// ── right column: reference ──────────────────────────────────────────────

	type irow struct {
		glyph string
		color lipgloss.Color
		desc  string
	}
	iconRows := []irow{
		{"◉", p.theme.Blue, "working"},
		{"⚠", p.theme.Gold, "needs input"},
		{"✓", p.theme.Green, "completed"},
		{"✗", p.theme.Red, "failed"},
		{"∙", p.theme.Overlay0, "idle"},
		{"○", p.theme.Overlay0, "stopped"},
	}

	var right []string
	right = append(right, "")
	right = append(right, sectionSt.Render("Icon Legend"))
	for _, ir := range iconRows {
		glyph := lipgloss.NewStyle().Foreground(ir.color).Bold(true).Render(ir.glyph)
		right = append(right, fmt.Sprintf("  %s  %s", glyph, descSt.Render(ir.desc)))
	}
	right = append(right, "")
	right = append(right, sectionSt.Render("Filter"))
	right = append(right, "  "+descSt.Render("type to match project · session · agent"))
	right = append(right, fmt.Sprintf("  %s%s%s",
		descSt.Render("case-insensitive · "),
		keySt.Render("esc"),
		descSt.Render(" to clear"),
	))
	right = append(right, "")
	right = append(right, sectionSt.Render("Config  ")+mauveSt.Render("~/.config/cleo/config.toml"))
	for _, key := range []string{
		"defaults.detach_key",
		"defaults.default_agent",
		"ui.theme",
		"ui.show_pane_preview",
		"agents.<name>",
	} {
		right = append(right, "  "+mauveSt.Render(key))
	}
	right = append(right, "")

	// ── pad shorter column ───────────────────────────────────────────────────

	for len(left) < len(right) {
		left = append(left, "")
	}
	for len(right) < len(left) {
		right = append(right, "")
	}

	// ── stitch ───────────────────────────────────────────────────────────────

	var b strings.Builder

	b.WriteString(bdr.Render(topBar) + "\n")

	// Title row: "Help" left-aligned in left cell, "esc / q to close" right-aligned in right cell.
	titleLeft := lipgloss.NewStyle().Foreground(p.theme.Accent).Bold(true).Render("Help")
	closeHint := lipgloss.NewStyle().Foreground(p.theme.Overlay0).Render("esc / q to close")
	gapClose := colW - lipgloss.Width(closeHint)
	if gapClose < 0 {
		gapClose = 0
	}
	b.WriteString(
		bdr.Render("│") + " " + padRight(titleLeft, colW) + " " +
			bdr.Render("│") + " " + strings.Repeat(" ", gapClose) + closeHint + " " +
			bdr.Render("│") + "\n",
	)
	b.WriteString(bdr.Render(midBar) + "\n")

	// Body rows.
	for i := range left {
		l := truncateWidth(left[i], colW)
		r := truncateWidth(right[i], colW)
		b.WriteString(
			bdr.Render("│") + " " + padRight(l, colW) + " " +
				bdr.Render("│") + " " + padRight(r, colW) + " " +
				bdr.Render("│") + "\n",
		)
	}

	b.WriteString(bdr.Render(botBar))
	return b.String()
}
```

- [ ] **Step 2: Run the new test to confirm it passes**

```bash
go test ./internal/tui/ -run TestHelpPopupView -v
```

Expected output:
```
=== RUN   TestHelpPopupView
--- PASS: TestHelpPopupView (0.00s)
PASS
```

- [ ] **Step 3: Run the full TUI test suite to confirm no regressions**

```bash
go test ./internal/tui/ -v -timeout 30s
```

Expected: all tests PASS, including `TestEscClosesPopupOnly` and `TestStatusClearsOnPopupOpen/help` which exercise `NewHelpPopup`.

- [ ] **Step 4: Build to confirm no compile errors**

```bash
go build ./...
```

Expected: exits 0, no output.

- [ ] **Step 5: Commit**

```bash
git add internal/tui/popup_help.go internal/tui/tui_test.go
git commit -m "feat(tui): two-column help panel with icon legend, filter ref, config"
```

---

## Self-Review Checklist

- [x] Spec coverage: all sections covered — Navigation ✓, Session Actions ✓, Global ✓, tmux ✓, Icon Legend ✓, Filter ✓, Config ✓
- [x] No placeholders — all code is complete
- [x] `NewHelpPopup` constructor signature unchanged — `TestEscClosesPopupOnly` still compiles
- [x] `padRight` and `truncateWidth` already exist in `styles.go` — no new helpers needed
- [x] `catppuccinMocha` is package-level var in `themes.go` — accessible from test in same package
- [x] `p.theme.Mauve` is a valid `Theme` field — verified in `themes.go`
- [x] `p.theme.Blue` is a valid `Theme` field — verified in `themes.go`
