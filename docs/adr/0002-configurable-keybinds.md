---
status: accepted
---

# Configurable keybinds are owned and resolved by the config package

Users can rebind main-view keymap actions (navigation, opening/closing popups, session/project actions) via a `[keybinds]` table in `config.toml`. The `config` package — not `tui` — owns the action namespace, the default bindings, validation, and the resolved `Keymap` struct; `config` imports `bubbles/key` and exposes a ready-to-use `Keymap` that `tui` consumes read-only. Resolution runs once inside `Load()`, storing the `Keymap` on `Config` (`toml:"-"`) and appending any problems to the existing `c.Warnings`.

## Why

`tui` already imports `config` (e.g. `handle_key.go` reads `m.ctx.Config`), so `config` cannot import `tui` — yet the keymap defaults previously lived in `tui.DefaultKeymap()` while config validation (themes, clamps) lived in `config.Load()`. Splitting keybind defaults from keybind validation across that one-way dependency would have produced two warning paths and a split-brain over which package owns the contract. Giving `config` full ownership (defaults + namespace + validation + the resolved `Keymap`) makes `tui` a pure consumer (`km := m.ctx.Config.Keymap`), routes every warning through the one channel the boot popup reads, and matches the theme-fallback pattern already in `Load()`. It also retires a latent bug: `handle_key.go` rebuilt `DefaultKeymap()` on every keystroke; the resolved keymap is now computed once.

The config schema is `action -> list of keys` (mirroring `key.Binding`'s native multi-key support). Listing an action replaces its keys entirely; omitted actions keep defaults; an empty list reverts to default (there is no "disabled" state, so every action always resolves to ≥1 key). The canonical action list is an importance-ordered slice that triples as the validation namespace, the help-screen order, and the conflict-precedence ranking. On a key collision the earlier (more core) action keeps the key — **first-wins by importance** — and the loser keeps its remaining keys. `esc`, `ctrl+c`, and `enter` are reserved hatches: they always perform close/cancel, quit, and attach/confirm in every mode and cannot be reassigned, which removes the lockout risk and the cross-mode collision where a rebound `close` would fire while typing into the finder. Help popup and footer hints derive their labels from the resolved keymap (help shows all keys, footer shows the first), so there is a single source of truth.

## Considered and rejected

- **Leaf `internal/keys` package** owning the namespace, imported by both `config` and `tui` — cleaner separation, but a whole new package and indirection for one feature; the dependency-purity it buys is moot once `config` already imports `bubbles/key`.
- **`config` holds raw strings, `tui` validates and builds the `Keymap`** — produces warnings outside `Load()` that must be merged into the boot popup separately, and lets a second entry point (tests, a future headless mode) skip resolution and get an empty keymap.
- **Hard-error on conflict** — breaks the loader's "always boots with sane defaults" guarantee; rejected in favor of warn + first-wins surfaced in a boot popup.
- **Alphabetical or handler-order conflict precedence** — arbitrary and unrelated to how core an action is; the importance-ordered slice is the documented contract instead.
- **Empty list = intentionally disabled** — rejected for empty = revert-to-default, keeping the "every action has ≥1 key" invariant and simplifying help/footer rendering.

## Consequences

The action names, the `[keybinds]` schema, and the reserved-key set (`esc`/`ctrl+c`/`enter`) are a stable contract encoded in user config files; renaming an action breaks existing configs. `config` now depends on `github.com/charmbracelet/bubbles/key`. `Config` carries two representations of the same data — raw `Keybinds map[string][]string` (serialized) and the computed `Keymap` (`toml:"-"`). The help popup (`popup_help.go`) and footer hints (`view.go`), previously hand-maintained, must now be generated from the resolved keymap; within-popup keys (textinput editing, spawn-field tab cycling, finder query characters) remain hardcoded and out of scope.
