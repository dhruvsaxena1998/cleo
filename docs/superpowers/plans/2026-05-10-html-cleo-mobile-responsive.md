# html/cleo Mobile Responsive Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make `html/cleo/index.html` and `html/cleo/docs.html` polished on mobile down to 375px, fixing horizontal page scroll, awkward stacking, and cramped typography while preserving the desktop experience exactly.

**Architecture:** Pure CSS edits inside the existing single embedded `<style>` block at the bottom of each HTML file. Two new breakpoints introduced (`≤640px`, `≤380px`); existing `≤600px` block is migrated to `≤640px`. No new files, no JavaScript, no build step.

**Tech Stack:** Plain HTML + CSS (single-file, embedded styles). No framework, no preprocessor. Verification via Python's built-in `http.server` and a browser DevTools device emulator.

**Spec:** `docs/superpowers/specs/2026-05-10-html-cleo-mobile-responsive-design.md`

---

## Verification model (read once)

Standard TDD doesn't apply — visual CSS work is verified by eyeballing at target viewport widths and asserting the spec's acceptance criteria. Every task that changes CSS ends with:

1. Hard-reload the browser (`⌘⇧R`) at each named width (320, 375, 390, 414, 640, 980)
2. Switch color scheme via the in-page theme toggle (light + dark)
3. Confirm the task's acceptance criterion holds
4. Commit

Open DevTools → Toggle device toolbar (`⌘⇧M` in Chrome) for viewport width control.

The "no horizontal page scroll" criterion can also be checked in the console:

```javascript
document.scrollingElement.scrollWidth === document.scrollingElement.clientWidth
// must return true at every width
```

---

## Task 1: Set up dev server and ignore brainstorm artifacts

**Files:**
- Modify: `.gitignore`

- [ ] **Step 1: Add `.superpowers/` to `.gitignore`**

Append the line `.superpowers/` to `.gitignore`. The new file should read:

```
bin/
*.test
*.out
.DS_Store
.superpowers/
```

- [ ] **Step 2: Start a local file server in the background**

Run:

```bash
cd html/cleo && python3 -m http.server 8000 &
```

Verify by opening `http://localhost:8000/index.html` and `http://localhost:8000/docs.html` in a browser. Both should render exactly as they do today (we have not changed any CSS yet).

- [ ] **Step 3: Capture a baseline at 375px**

In DevTools → device toolbar, set width to 375px. Open both pages. Mentally note the obvious breakage:
- Horizontal page scroll on `index.html` (terminal frame's decorative `::before` paper layer)
- Accordion ASCII art clipping
- Install pill copy button hanging off the right edge
- Docs `<pre>` blocks may force page scroll

This baseline is for your reference — not committed.

- [ ] **Step 4: Commit gitignore**

```bash
git add .gitignore
git commit -m "chore: ignore .superpowers brainstorm artifacts"
```

---

## Task 2: index.html — global backstop and 600 → 640 breakpoint migration

**Files:**
- Modify: `html/cleo/index.html:100` (add `overflow-x: hidden`)
- Modify: `html/cleo/index.html:906` (rename media query from 600px to 640px)

- [ ] **Step 1: Add `overflow-x: hidden` to `html, body`**

Replace the line at `html/cleo/index.html:100`:

```css
html, body { margin: 0; padding: 0; }
```

with:

```css
html, body { margin: 0; padding: 0; overflow-x: hidden; }
```

- [ ] **Step 2: Migrate the 600px breakpoint to 640px**

At `html/cleo/index.html:906`, change:

```css
@media (max-width: 600px) {
```

to:

```css
@media (max-width: 640px) {
```

- [ ] **Step 3: Verify**

Reload `index.html` at width 375px. The horizontal page scroll caused by the `term-frame::before` overflow should be gone (clipped by the body). Switch to width 980px+ and verify desktop is unchanged.

- [ ] **Step 4: Commit**

```bash
git add html/cleo/index.html
git commit -m "fix(landing): backstop overflow-x and migrate 600px breakpoint to 640px"
```

---

## Task 3: index.html — Strategy B (terminal showcase horizontal scroll + edge fade)

**Files:**
- Modify: `html/cleo/index.html:915-917` (replace pane-stacking rules with horizontal scroll)
- Modify: `html/cleo/index.html:368` (`.term-body` definition — add right-edge fade as a sibling rule)

- [ ] **Step 1: Replace the pane-stacking rules at the 640px breakpoint**

Inside the `@media (max-width: 640px)` block, find these three lines (currently at `html/cleo/index.html:915-917`):

```css
  .term-body { grid-template-columns: 1fr; }
  .term-pane { border-right: none; border-bottom: 1px solid color-mix(in oklch, var(--term-bg), white 8%); padding: 14px 16px; }
  .term-pane:last-child { border-bottom: none; }
```

Replace them with:

```css
  .term-body { overflow-x: auto; -webkit-overflow-scrolling: touch; scrollbar-width: thin; }
  .term-pane { padding: 14px 16px; min-width: 280px; }
```

This keeps the two-pane grid intact but lets the `.term-body` scroll horizontally inside its rounded frame. `min-width: 280px` ensures each pane stays usable when scrolled to.

- [ ] **Step 2: Add a right-edge fade gradient to hint at scroll affordance**

The fade lives on `.term` (not `.term-body`) because at narrow widths `.term-body` becomes the horizontal-scroll container — a `::after` positioned within it would resolve `right: 0` against the scroll-end, not the visible edge. `.term` already has `position: relative` and `overflow: hidden`, which is exactly what we need.

Immediately after the existing `.term { ... }` rule (it ends around `html/cleo/index.html:320`), add a new rule:

```css
.term::after {
  content: "";
  position: absolute;
  top: 0; right: 0; bottom: 0;
  width: 28px;
  background: linear-gradient(90deg, transparent, var(--term-bg));
  pointer-events: none;
  opacity: 0;
  transition: opacity 160ms ease;
  z-index: 2;
}
@media (max-width: 640px) {
  .term::after { opacity: 1; }
}
```

The fade is invisible at desktop width (where there's no inner scroll) and appears only when the horizontal-scroll behavior is active. It overlays the rightmost 28px of the visible terminal card and stays anchored to the visible edge regardless of how far the user has swiped the body.

- [ ] **Step 3: Verify**

Reload `index.html` at 375px:
- The terminal mock should no longer stack panes — instead, the right pane is partially hidden and reachable by horizontal swipe inside the dark frame
- A subtle right-edge fade should be visible
- Page itself should not scroll horizontally
- `document.scrollingElement.scrollWidth === document.scrollingElement.clientWidth` returns `true`

At 980px+, the terminal renders in two panes side-by-side with no fade, exactly as before.

- [ ] **Step 4: Commit**

```bash
git add html/cleo/index.html
git commit -m "feat(landing): horizontal-scroll terminal showcase on mobile with edge fade"
```

---

## Task 4: index.html — Strategy C (hide accordion ASCII at narrow widths)

**Files:**
- Modify: `html/cleo/index.html:906-929` (inside the `@media (max-width: 640px)` block)

- [ ] **Step 1: Hide accordion ASCII art at 640px**

Inside the `@media (max-width: 640px)` block, find the existing line:

```css
  .acc-body .ascii { font-size: 11px; }
```

Replace it with:

```css
  .acc-body .ascii { display: none; }
```

The accordion body becomes prose-only on mobile. The existing `.acc-body { padding: 0 18px 22px 18px; }` rule already collapses the body to single column.

- [ ] **Step 2: Verify**

Reload `index.html` at 375px. Click each of the five "How it works" accordion items. Confirm:
- All five items still expand and collapse with no JS errors
- Each open body shows just the prose paragraph(s) — no ASCII block
- Body text is comfortably readable, no overflow inside the accordion card

At 980px, accordion bodies still show their ASCII art alongside the prose.

- [ ] **Step 3: Commit**

```bash
git add html/cleo/index.html
git commit -m "feat(landing): prose-only accordion on mobile, hide ASCII art ≤640px"
```

---

## Task 5: index.html — nav, hero, install pill at narrow widths

**Files:**
- Modify: `html/cleo/index.html:884` (hero h1 clamp inside 980px block)
- Modify: `html/cleo/index.html:906-929` (inside 640px block — install pill stacking)
- Add: new `@media (max-width: 380px)` block after the 640px block (~line 929)

- [ ] **Step 1: Tighten hero h1 clamp**

At `html/cleo/index.html:884`, change:

```css
  .hero h1 { font-size: clamp(40px, 8vw, 60px); }
```

to:

```css
  .hero h1 { font-size: clamp(34px, 9vw, 60px); }
```

This widens the responsive range so 375px viewports get a 34px h1 instead of 40px.

- [ ] **Step 2: Stack the install pill vertically inside the 640px block**

Inside the `@media (max-width: 640px)` block, find these existing rules:

```css
  .install-pill { font-size: 13px; }
  .install-pill .prompt { padding: 12px 12px 12px 14px; }
  .install-pill code { padding: 12px 14px; }
  .install-pill button { padding: 0 14px; }
```

Replace them with:

```css
  .install-pill {
    display: flex;
    flex-direction: column;
    align-items: stretch;
    font-size: 13px;
  }
  .install-pill .prompt {
    padding: 10px 14px;
    border-right: none;
    border-bottom: 1px solid var(--hairline);
    text-align: left;
  }
  .install-pill code {
    padding: 12px 14px;
    white-space: nowrap;
    overflow-x: auto;
  }
  .install-pill button {
    border-left: none;
    border-top: 1px solid var(--hairline);
    padding: 12px 14px;
    text-align: left;
  }
```

This makes the pill a vertical stack: prompt → command → COPY button. The command line gets `overflow-x: auto` so a long npm command can scroll inside the pill without forcing page scroll.

- [ ] **Step 3: Add a new ≤380px block for nav compaction**

Immediately after the closing `}` of the `@media (max-width: 640px)` block (i.e. before the existing `@media (prefers-reduced-motion: reduce)` block), insert:

```css
@media (max-width: 380px) {
  .container { padding: 0 16px; }
  .nav-inner { padding: 12px 16px; gap: 10px; }
  .wordmark { font-size: 20px; }
  .cat-mark { width: 24px; height: 24px; }
  .nav-version { display: none; }
  .nav-links { gap: 10px; }
  .nav-links .gh { padding: 6px 10px; gap: 6px; font-size: 12px; }
  .feature { padding: 22px 22px; }
}
```

This compacts the nav so the wordmark, theme toggle, and "Star" button all fit at 320–375px without wrapping. The version pill is hidden because it's redundant decoration. Features padding tightens.

- [ ] **Step 4: Verify**

Reload `index.html`. At each width:
- **320px:** Nav fits in one row, no wrap; theme toggle and Star button visible. Hero h1 is readable, doesn't wrap mid-word. Install pill is vertically stacked.
- **375px:** Same as above. Install command line scrolls horizontally inside the pill if too long; pill button visible at bottom.
- **640px:** Nav still has theme toggle + Star button. Install pill stacks vertically.
- **980px:** Desktop unchanged — install pill is horizontal, nav has all links, no compaction.

- [ ] **Step 5: Commit**

```bash
git add html/cleo/index.html
git commit -m "feat(landing): tighten hero, stack install pill, compact nav at ≤380px"
```

---

## Task 6: index.html — final landing-page sweep

- [ ] **Step 1: Walk every named width on `index.html` in light mode**

For each of `320, 375, 390, 414, 640, 980` px, with light mode active:
- Open `http://localhost:8000/index.html`
- Run `document.scrollingElement.scrollWidth === document.scrollingElement.clientWidth` in console — must be `true`
- Scroll the page top-to-bottom; eyeball each section against the spec's acceptance criteria
- Note any breakage in a scratch list

- [ ] **Step 2: Walk every named width in dark mode**

Click the theme toggle. Repeat the sweep. Catch any dark-mode-only contrast or border issues introduced by the new rules.

- [ ] **Step 3: Functional smoke**

At 375px:
- Click "COPY" on the install pill → button text changes to indicate copied
- Click each accordion summary → opens/closes; prose body visible; ASCII art absent
- Click each TUI palette switcher pill → terminal mock recolors live
- Click theme toggle → light/dark transitions, persists on reload

- [ ] **Step 4: Fix any issues found**

If the sweep surfaces issues, edit the CSS, reload, repeat. Commit each fix as a separate `fix(landing): ...` commit so review history stays granular.

- [ ] **Step 5: Confirm baseline before moving on**

When the sweep is clean (zero criterion violations, zero functional regressions), proceed to Task 7. The expected commit count for index.html so far: ~5 commits (one per task plus any fixups).

---

## Task 7: docs.html — global backstop, breakpoint migration, nav compaction

**Files:**
- Modify: `html/cleo/docs.html:89` (`html, body` backstop)
- Modify: `html/cleo/docs.html:370` (rename 600 → 640)
- Add: new `@media (max-width: 380px)` block after the 640px block

- [ ] **Step 1: Add `overflow-x: hidden` backstop**

At `html/cleo/docs.html:89`, change:

```css
html, body { margin: 0; padding: 0; }
```

to:

```css
html, body { margin: 0; padding: 0; overflow-x: hidden; }
```

- [ ] **Step 2: Migrate 600px breakpoint to 640px**

At `html/cleo/docs.html:370`, change:

```css
@media (max-width: 600px) {
```

to:

```css
@media (max-width: 640px) {
```

- [ ] **Step 3: Add ≤380px block with the same nav-compaction rules used on the landing page**

Immediately after the closing `}` of the `@media (max-width: 640px)` block on docs.html (currently around line 375), insert:

```css
@media (max-width: 380px) {
  .container { padding: 0 16px; }
  .nav-inner { padding: 12px 16px; gap: 10px; }
  .wordmark { font-size: 20px; }
  .cat-mark { width: 24px; height: 24px; }
  .nav-version { display: none; }
  .nav-links { gap: 10px; }
  .nav-links .gh { padding: 6px 10px; gap: 6px; font-size: 12px; }
}
```

- [ ] **Step 4: Verify**

Reload `docs.html` at 320, 375, 640, 980. Nav compacts identically to `index.html`. No horizontal page scroll at 375.

- [ ] **Step 5: Commit**

```bash
git add html/cleo/docs.html
git commit -m "fix(docs): backstop overflow-x, migrate 600→640, compact nav at ≤380px"
```

---

## Task 8: docs.html — hero clamp and content prose typography

**Files:**
- Modify: `html/cleo/docs.html:176-182` (`.docs-hero h1` clamp)
- Modify: `html/cleo/docs.html:227-233` (`.content h2`)
- Modify: `html/cleo/docs.html:241-247` (`.content h3`)
- Modify: `html/cleo/docs.html:225` (`.content section { margin-bottom: 72px; ... }`)

- [ ] **Step 1: Tighten `.docs-hero h1` clamp**

At `html/cleo/docs.html:176-182`, change:

```css
.docs-hero h1 {
  font-family: var(--font-display);
  font-size: clamp(36px, 4.6vw, 60px);
  line-height: 1.04; letter-spacing: -0.02em;
  font-weight: 500;
  margin: 18px 0 16px;
}
```

to:

```css
.docs-hero h1 {
  font-family: var(--font-display);
  font-size: clamp(30px, 9vw, 60px);
  line-height: 1.04; letter-spacing: -0.02em;
  font-weight: 500;
  margin: 18px 0 16px;
}
```

- [ ] **Step 2: Make `.content h2` fluid**

At `html/cleo/docs.html:227-233`, change:

```css
.content h2 {
  font-family: var(--font-display);
  font-size: 36px; line-height: 1.1;
  letter-spacing: -0.012em; font-weight: 500;
  margin: 0 0 8px;
  position: relative;
}
```

to:

```css
.content h2 {
  font-family: var(--font-display);
  font-size: clamp(28px, 5.5vw, 36px); line-height: 1.1;
  letter-spacing: -0.012em; font-weight: 500;
  margin: 0 0 8px;
  position: relative;
}
```

- [ ] **Step 3: Make `.content h3` fluid**

At `html/cleo/docs.html:241-247`, change:

```css
.content h3 {
  font-family: var(--font-display);
  font-size: 22px; line-height: 1.2;
  margin: 36px 0 10px;
  font-weight: 500;
  scroll-margin-top: 92px;
}
```

to:

```css
.content h3 {
  font-family: var(--font-display);
  font-size: clamp(19px, 4.5vw, 22px); line-height: 1.2;
  margin: 36px 0 10px;
  font-weight: 500;
  scroll-margin-top: 92px;
}
```

- [ ] **Step 4: Tighten section spacing inside the 640px block**

Inside the `@media (max-width: 640px)` block (around `html/cleo/docs.html:370-375`), append a new line:

```css
  .content section { margin-bottom: 48px; }
  .docs-hero { padding: 32px 0 12px; }
```

(The existing `.docs-hero { padding: 40px 0 12px; }` rule lives in the 980px block; this 640px rule overrides it for tighter phones.)

- [ ] **Step 5: Verify**

Reload `docs.html` at 320, 375, 414, 640, 980:
- Headings shrink fluidly; no awkward line-breaks mid-word
- Section vertical rhythm feels tighter on phones, normal on desktop
- Long inline `<code>` words wrap if needed (browser default, already in place)

- [ ] **Step 6: Commit**

```bash
git add html/cleo/docs.html
git commit -m "feat(docs): fluid hero/content heading clamps and tighter section rhythm"
```

---

## Task 9: docs.html — TOC, code blocks, command table, alerts

**Files:**
- Modify: `html/cleo/docs.html:360-368` (980px block — TOC card already lives here)
- Modify: inside `@media (max-width: 640px)` block (currently lines 370-375)
- Modify: inside the new `@media (max-width: 380px)` block added in Task 7

- [ ] **Step 1: Tighten TOC card and shrink nested indent at 640px**

Inside the `@media (max-width: 640px)` block, append:

```css
  .toc { padding: 14px 16px; }
  .toc ol ol { margin: 4px 0 6px 8px; padding-inline-start: 8px; }
```

- [ ] **Step 2: Shrink code-block padding and font at 640px**

Inside the same `@media (max-width: 640px)` block, append:

```css
  .content pre { padding: 14px 16px; font-size: 12.5px; }
```

- [ ] **Step 3: Tighten `.cmd-row` and `.alert` padding at 380px**

Inside the `@media (max-width: 380px)` block (added in Task 7), append:

```css
  .cmd-row { padding: 12px 14px; }
  .alert { grid-template-columns: 1fr; gap: 8px; padding: 12px 14px; }
  .alert .ico {
    display: inline-flex;
    width: auto; height: auto;
    border: none;
    margin-right: 4px;
    padding: 0;
  }
```

The `.alert` change drops the 36px icon column at very narrow widths and lets the icon render inline; the icon is already a single character (e.g. `i`, `!`) so no layout artifacts.

- [ ] **Step 4: Verify**

Reload `docs.html`:
- **375px:** TOC card padding feels right, nested subsection list does not double-wrap. Code blocks have visibly tighter padding. Command table rows are single-column, comfortably padded.
- **320px:** Alert icon inlines with text; no boxy 36px column.
- **980px:** No regressions — TOC sidebar is sticky, command table rows are two-column.

- [ ] **Step 5: Commit**

```bash
git add html/cleo/docs.html
git commit -m "feat(docs): tighten TOC, code blocks, command rows, and alerts at narrow widths"
```

---

## Task 10: Final cross-page verification sweep

- [ ] **Step 1: Run the full criteria sweep on both pages**

For each page (`index.html`, `docs.html`), each width (`320, 375, 390, 414, 640, 980`), and each color scheme (light, dark) — 24 viewports per page — confirm every acceptance criterion from the spec:

1. `document.scrollingElement.scrollWidth === clientWidth` returns `true` (no horizontal page scroll)
2. No clipped content other than intentional in-card horizontal scroll on the terminal mock and code blocks
3. All five accordion items expand; ASCII art hidden at ≤640px
4. Terminal showcase scrolls horizontally inside its frame at ≤640px with right-edge fade visible
5. Theme toggle and TUI palette switcher reachable and tappable
6. Hero h1, section h2, accordion title — no mid-character breaks
7. Light + dark both polished
8. JS still works: accordion, theme toggle persistence, copy button, palette switcher

- [ ] **Step 2: Capture findings**

Make a scratch list of any criterion violations. For each, write a small fix and commit as `fix(landing|docs): <what>`.

- [ ] **Step 3: Final clean check**

Re-walk the sweep one more time after fixes. All 8 criteria must hold for both pages at all 6 widths in both color schemes.

- [ ] **Step 4: Stop the dev server**

```bash
# Find and kill the http.server process
lsof -ti :8000 | xargs kill 2>/dev/null
```

- [ ] **Step 5: Final commit if anything was fixed in step 2**

If step 2 found nothing, no commit needed. Otherwise the fixups are already committed individually.

---

## Done criteria

- All 10 tasks have their checkboxes ticked
- The branch contains commits for: `.gitignore`, index.html (multiple), docs.html (multiple)
- The full verification sweep passes on both pages at 6 widths in 2 color schemes
- Desktop (≥980px) is byte-for-byte equivalent in observable rendering to `main`'s version (we only added rules inside `≤640px` and `≤380px` blocks, plus a globally-applied `overflow-x: hidden` on `html, body` and an invisible `::after` on `.term-body`)

After this plan executes, the next step is to open a PR back to `main` from this worktree's branch.
