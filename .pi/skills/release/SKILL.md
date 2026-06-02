---
name: release
description: Bump cleo version, update changelog, docs markdown status, and landing-page version references, then tag and push a release. Use when user says "release", "new version", "tag", "ship", "bump version", or after merging features for a release.
---

# Release cleo

## Quick start

```bash
# After all release changes are committed:
git tag -a vX.Y.Z -m "vX.Y.Z: <summary>" && git push origin vX.Y.Z
```

GitHub Actions (`.github/workflows/release.yml`) runs goreleaser on pushed `v*` tags, builds binaries, creates the GitHub Release, and updates the Homebrew tap.

## Current release context

- Work must not happen directly on `main`. If the checkout is on `main`, create a feature branch before editing or committing.
- GitHub-rendered markdown is the documentation source of truth. See `docs/adr/0003-github-as-docs-source-of-truth.md`.
- `README.md` is now a slim docs hub with the current status line and deep links to `docs/` pages.
- User documentation lives in `docs/installation.md`, `docs/quickstart.md`, `docs/commands.md`, `docs/configuration.md`, `docs/hooks.md`, `docs/troubleshooting.md`, and `docs/aliases.md`.
- `html/cleo/docs.html` has been deleted. Do not recreate it and do not add release-time sync steps for it.
- `html/cleo/index.html` is the public marketing landing page. It is static, so release work must manually update version badges, install examples, docs links, terminal-demo copy, feature cards, and representative default footer keys.
- Recent feature work made keybindings configurable and dynamic. Do not copy key lists from memory. Use the code defaults as source of truth:
  - `internal/config/keymap.go` - default actions, key order, conflict precedence, reserved keys
  - `internal/config/defaults.go` and `internal/config/schema.go` - config defaults such as `[ui].editor`
  - `internal/tui/keyhint.go`, `internal/tui/popup_help.go`, `internal/tui/view.go` - dynamic footer/help rendering
- Recent docs gotchas to audit on every release: editor action (`ctrl+g` or `e`), send action (`m`), mute action (`alt+m`), `[keybinds]` validation and reserved hatches, boot warnings popup, and footer/help text deriving from the resolved keymap.

## Process

**Step 0 - determine version.** Always start by finding the latest tag and asking the user what the next version should be:

```bash
git tag --sort=-version:refname | head -3
```

Present the latest tag, summarize unreleased changes from `git log <last-tag>..HEAD`, and ask: *"What should the next version be?"* Suggest the likely semver bump. Do not proceed until the user confirms a version.

See [CHECKLIST.md](CHECKLIST.md) for the full step-by-step. High-level order:

1. **Preflight** - branch off `main` if needed, inspect dirty files, find latest tag, review git logs since that tag
2. **Version** - bump `internal/cli/root.go`
3. **Changelog** - add `[X.Y.Z]` section to `CHANGELOG.md`
4. **Version refs** - update current release references in `README.md` and `html/cleo/index.html`
5. **Markdown docs source** - verify relevant `docs/` pages already reflect feature work; do not duplicate-sync a deleted `docs.html`
6. **Landing page** - update `html/cleo/index.html` install examples, docs links, TUI demo, footer, and feature copy
7. **Surface audit** - config schema, CLI commands, defaults, hooks, keymap, screenshots/demo copy, and outbound links
8. **Commit** - `"chore: bump version to vX.Y.Z, update docs and changelog"`
9. **Tag** - `git tag -a vX.Y.Z` with release notes summary
10. **Push** - branch/PR first, then push tag after merge; tag triggers goreleaser

## Key files

| File | What to update |
|---|---|
| `internal/cli/root.go` | `Version` constant |
| `CHANGELOG.md` | New `[X.Y.Z]` section based on commits since latest tag |
| `README.md` | Status line and docs index links if paths changed |
| `docs/*.md` | Source-of-truth user docs, edited with feature work; audit for stale commands/config/keybinds |
| `docs/adr/0003-github-as-docs-source-of-truth.md` | Docs architecture decision, if docs publishing changes |
| `html/cleo/index.html` | Landing page: version refs, GitHub docs links, install commands, terminal demo, default footer keys, features |
| `internal/config/defaults.go` | Config defaults and bundled agents |
| `internal/config/keymap.go` | Default keybindings, validation namespace, reserved hatches |
| `internal/config/schema.go` | Config fields users can set |
| `internal/tui/keyhint.go` / `popup_help.go` / `view.go` | Live footer/help behavior that docs should describe |
