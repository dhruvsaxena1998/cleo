# Release Checklist

## 0. Preflight and version

Confirm branch and dirty state first:

```bash
git status --short --branch
```

If on `main`, create a release branch before editing or committing:

```bash
git switch -c release/vX.Y.Z
```

Find the latest tag:

```bash
git tag --sort=-version:refname | head -3
latest_tag=$(git describe --tags --abbrev=0)
```

Review unreleased changes, especially docs and UI-surface changes:

```bash
git log --oneline --decorate "$latest_tag"..HEAD
git log --oneline --stat "$latest_tag"..HEAD -- \
  README.md CHANGELOG.md docs html/cleo/index.html \
  internal/cli internal/config internal/tui .github scripts
```

Ask the user what version to release next. Print the latest tag and suggest the likely semver bump:

- Patch (`v0.1.2` to `v0.1.3`) for bug fixes only
- Minor (`v0.1.2` to `v0.2.0`) for new features
- Major (`v0.1.2` to `v1.0.0`) for breaking changes

Do not proceed until the user confirms a specific version number.

## 1. Bump version

Edit `internal/cli/root.go`:

```go
var Version = "X.Y.Z"
```

## 2. Changelog

Edit `CHANGELOG.md`. Add a new `[X.Y.Z] - YYYY-MM-DD` section above the previous release. Pull features/fixes from:

```bash
git log "$latest_tag"..HEAD --format="%h %s%n%b"
```

Use [Keep a Changelog](https://keepachangelog.com/en/1.1.0/) format:

```markdown
## [X.Y.Z] - YYYY-MM-DD

### Added
- Feature one.

### Changed
- Thing changed.

### Fixed
- Bug fixed.
```

Move applicable bullets out of `[Unreleased]` into the release section.

## 3. Version references

Update current-release version strings:

| File | Locations |
|---|---|
| `internal/cli/root.go` | `Version` constant without leading `v` |
| `README.md` | Status line: `> **Status:** vX.Y.Z` |
| `html/cleo/index.html` | Nav badge `.nav-version`, hero eyebrow, install verify output |

Run to find stragglers:

```bash
rg "v<old-version>|<old-version>" README.md CHANGELOG.md docs html/cleo internal/cli/root.go
```

Avoid changing historical changelog headings and old design docs unless they are active current-release copy. `html/cleo/docs.html` is deleted by ADR 0003 and must not be recreated.

## 4. Markdown docs source audit: `docs/`

GitHub-rendered markdown under `docs/` is the source of truth. See `docs/adr/0003-github-as-docs-source-of-truth.md`.

Release work should not hand-sync a second docs copy. Instead, verify that feature work already updated the relevant source pages:

| Source page | Audit when |
|---|---|
| `docs/installation.md` | Install, verify, upgrade, uninstall, release artifact behavior changed |
| `docs/quickstart.md` | TUI workflow, dashboard keys, first-run flow changed |
| `docs/commands.md` | CLI command, flag, hook setup, or examples changed |
| `docs/configuration.md` | Config schema, defaults, agents, themes, keybinds, recipes changed |
| `docs/hooks.md` | Hook protocol, supported agents, state transitions, managed files changed |
| `docs/troubleshooting.md` | Error messages, doctor checks, recovery recipes changed |
| `docs/aliases.md` | Alias behavior changed |

Useful code/source checks:

```bash
rg -n "new[A-Z].*Cmd|Use:|Short:|Long:|Flags\(" internal/cli
rg -n "type Config|type UI|type Tmux|type Sound|type Timeouts|type Pruning|Keybinds" internal/config
rg -n "Defaults_|Theme|Editor|SidebarWidth|EventLogLines|PanePreview|Agents" internal/config/defaults.go internal/config/schema.go
rg -n "Protocol|Install|Cleanup|doctor|hooks" internal
```

## 5. Keybinding audit

Keybindings are configurable and the TUI footer/help derives labels from the resolved keymap. Do not document keybindings from memory.

```bash
rg -n "keybindActions|reservedKeys|specialKeys|resolveKeymap" internal/config/keymap.go
rg -n "KeyHint|Help|footer|keymap|Binding" internal/tui
```

Keep these concepts accurate in `docs/configuration.md`, `docs/quickstart.md`, and the landing-page demo:

- Defaults are configurable through `[keybinds]`.
- `enter`, `esc`, and `ctrl+c` are reserved hatches.
- Invalid keys are dropped with warnings; an action with no valid override falls back to defaults.
- Conflicts resolve by action order, first-wins.
- The help popup and footer derive labels from the resolved keymap.
- Popup-internal editing keys are not configurable.
- Current gotchas: editor is `ctrl+g` or `e`, send is `m`, mute is `alt+m`, kill is `K` or `ctrl+k`, remove project is `D`.

## 6. Landing page audit: `html/cleo/index.html`

The landing page is static and easy to drift. Review at least:

- Version refs: nav badge, hero eyebrow, install verify output.
- Docs links: links should point into GitHub markdown docs or the README docs index, not to deleted `docs.html`.
- Install flow: curl installer, Homebrew, hook setup, supported agents, uninstall commands.
- Terminal demo: project/session labels, states, footer keys, action names, event names.
- Feature cards: hooks support, sound behavior, local-first paths, key examples.
- Copy that names defaults: derive from `internal/config/defaults.go` and `internal/config/keymap.go`.

Check landing links before release:

```bash
rg -n "href=|docs\.html|troubleshooting|#documentation|docs/|releases|CHANGELOG" html/cleo/index.html
```

## 7. Global docs and link checks

Search for old commands, old versions, and stale links:

```bash
rg -n "cleo init|cleo hook |ctrl-k|ctrl\+k|alt\+m|ctrl\+g|docs\.html|v<old-version>" \
  README.md CHANGELOG.md docs html/cleo .pi/skills/release

rg -n "\]\(([^)]*)\)" README.md docs | head
```

If a path is linked, make sure the file exists. Do not treat historical mentions inside ADRs and old `docs/superpowers/` plans as release blockers unless they are linked as user docs.

## 8. Build, test, and smoke check

Run the normal test suite and a version smoke check:

```bash
go test ./...
make build
./bin/cleo --version
```

If HTML changed, open or preview the landing page enough to catch broken layout or copy mistakes.

## 9. Commit

```bash
git add CHANGELOG.md README.md docs html/cleo/index.html internal/cli/root.go
# add any other changed files
git commit -m "chore: bump version to vX.Y.Z, update docs and changelog

- Version: <old> to <new> in cli, landing page, README
- CHANGELOG: add vX.Y.Z section
- Docs: <notable docs/link/config/keybind updates>"
git push -u origin HEAD
```

Open a pull request. Do not push directly to `main`.

## 10. Tag after merge

After the PR is merged and `main` is up to date:

```bash
git switch main
git pull --ff-only
git tag -a vX.Y.Z -m "vX.Y.Z: <brief summary of headliners>"
git push origin vX.Y.Z
```

GitHub Actions (`release.yml`) triggers on `v*` tags and runs goreleaser:

- Builds binaries for macOS (arm64, amd64) and Linux (arm64, amd64)
- Creates GitHub Release with artifacts
- Updates Homebrew formula in `dhruvsaxena1998/homebrew-tap`

Verify the release at: `https://github.com/dhruvsaxena1998/cleo/releases`
