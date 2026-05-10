# html/cleo Mobile Responsive Design Spec

**Date:** 2026-05-10
**Status:** Draft (pending user review)
**Scope:** `html/cleo/index.html` and `html/cleo/docs.html`
**Workspace:** new git worktree from `main`

## Goal

Make the cleo landing page and documentation page feel polished on mobile — no horizontal page scroll, no awkward stacking, readable typography — while preserving the desktop experience exactly as it is today.

## Non-goals

- HTML structural refactor, CSS deduplication, or extracting shared styles between the two files (deferred to a future spec)
- New content, new sections, or visual redesign at desktop widths
- Adding JavaScript (other than what's already there for the accordion, theme toggle, copy button, and TUI palette switcher)
- A build step, minifier, or templating system
- Screenshots, animation work, or font-loading changes

## Constraints (user-set)

- **Narrowest target width:** 375px (iPhone SE), with 320px treated as a stress-test floor
- **Keep visible on mobile:** TUI theme switcher (palette picker), light/dark theme toggle, all five "How it works" accordion steps
- **Comprehensive polish bar:** real per-section treatment, validate at every named width

## Approach

### Where new CSS lives

All responsive rules continue to live inside the existing single embedded `<style>` block at the bottom of each file, in the `/* ---------- responsive ---------- */` section. We do not introduce additional stylesheets, classes, or markup.

### Breakpoint plan

| Breakpoint | Role |
|---|---|
| `> 980px` | Desktop — untouched |
| `≤ 980px` | Tablet — current rules retained, expanded for overflow fixes |
| `≤ 640px` | New intermediate breakpoint — replaces the existing 600px boundary; covers small tablet through large phone |
| `≤ 380px` | New tight-phone breakpoint — used sparingly for hero h1 sizing, install pill stacking, alert icon collapse, nav compaction |

### Strategies for intrinsically wide content

- **Strategy B — terminal showcase:** at ≤640px, the `.term` becomes a horizontal-scroll container (`overflow-x: auto` on `.term-body`, `.term-status`, `.footer-line`) with a right-edge fade gradient. The two-pane grid is preserved so a swipe reveals the right pane. The `.term-frame::before` decorative paper layer's negative-inset is neutralized at ≤980px so it stops causing page-level overflow.
- **Strategy C — accordion ASCII art:** at ≤640px, `.acc-body .ascii { display: none; }`. Bodies render prose-only in single column. All five steps remain visible and expandable.

### Global backstop

`html, body { overflow-x: hidden; }` to prevent any future overflow leaks (decorative pseudo-elements, etc.) from triggering page-level horizontal scroll.

---

## index.html — per-section treatment

| Section | Treatment |
|---|---|
| **Nav** | At ≤640px, hide non-active middle links (existing rule, boundary moved). At ≤380px: wordmark `25px → 20px`, version pill compacts, `gh` button drops the icon-padding and reads as a tighter "Star" pill. Theme toggle stays. |
| **Hero** | `h1` clamp tightened to `clamp(34px, 9vw, 60px)`. Hero meta row already wraps via existing `flex-wrap`. Install pill stacks vertically (`prompt`, `code`, `button`) at ≤640px so the button no longer hangs off the right edge. |
| **Terminal showcase** | Strategy B (above). Caption RHS already wraps. TUI theme switcher (`.tui-switcher`) stays; existing `flex-wrap` is enough. The `.tui-pick` font/padding compaction at ≤980px is retained. |
| **How it works (accordion)** | Strategy C (above). `.acc-summary` collapses to `1fr 24px` at ≤640px (current rule, boundary moved). `.acc-num` metadata stays hidden as it already does. |
| **Features** | Already collapses to one column at ≤980px. At ≤380px: per-feature padding `28px → 22px`, gap tightens. |
| **Install section** | Already collapses at ≤980px. CTA row wraps via existing `flex-wrap`. No additional rules. |
| **Footer** | Already collapses 4-col → 2-col → 1-col across existing breakpoints. No additional rules. |

## docs.html — per-section treatment

| Section | Treatment |
|---|---|
| **Nav** | Same component as `index.html` — same compaction at ≤640px and ≤380px. |
| **Docs hero** | `h1` clamp tightened: `clamp(30px, 9vw, 60px)`. Hero padding `64px → 32px` at ≤640px. |
| **TOC + content grid** | Already collapses to one column at ≤980px and renders TOC as a card (existing rule). At ≤640px the TOC card padding tightens (`18px → 14px`) and the nested `<ol ol>` indent shrinks so the subsection list does not double-wrap. |
| **Content prose** | `max-width: 68ch` retained (already fluid). `.content h2` `36px → clamp(28px, 5.5vw, 36px)`. `.content h3` `22px → clamp(19px, 4.5vw, 22px)`. Section `margin-bottom 72px → 48px` at ≤640px. |
| **Code blocks (`pre`)** | Existing `overflow-x: auto` retained. At ≤640px: padding `18px 22px → 14px 16px`, font `13px → 12.5px`. |
| **Command table (`.cmd-row`)** | Already collapses to single column at ≤600px (boundary moved to 640px). At ≤380px: per-row padding tightens. |
| **Alerts** | Grid `36px 1fr` retained at ≥380px. At ≤380px: drop the icon column and prepend the icon glyph inline with the body text. |
| **Footer** | `.footer-tail-min` already wraps via existing `flex-wrap`. No additional rules. |

---

## Verification

### Manual viewport sweep

Both pages, in both color schemes (light + dark), at: **320px, 375px, 390px, 414px, 640px, 980px**.

### Acceptance criteria — must hold at every width above

1. No horizontal page scroll: `document.scrollingElement.scrollWidth === clientWidth`.
2. No clipped content other than intentional in-card horizontal scroll on the terminal mock and code blocks.
3. All five accordion items expand and the prose body is readable; ASCII art is hidden at ≤640px.
4. Terminal showcase scrolls horizontally inside its frame at ≤640px with a visible right-edge fade. Theme switcher remains functional.
5. Theme toggle and TUI palette switcher are reachable and tappable at every width.
6. Hero `h1`, section `h2`, and accordion title never wrap mid-character or break awkwardly.
7. Light mode and dark mode both look polished — no contrast losses, no orphaned dark-mode bugs from new rules.
8. No JS regressions — accordion open/close, theme toggle persistence, copy-install button, and TUI palette switching all still work.

### How verification will be performed

- Local file server (no build required) opened in a browser with DevTools device emulator
- Each width walked manually on each page in both color schemes
- Issue list captured per breakpoint before declaring done
- Optional Playwright assertion of criterion #1 (no horizontal scroll) is **out of scope for this pass** unless the user requests it explicitly

## Open questions

None at the time of writing.
