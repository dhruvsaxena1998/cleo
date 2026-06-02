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
  README.md CHANGELOG.md html/cleo/index.html html/cleo/docs.html \
  internal/cli internal/config internal/tui docs .github scripts
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

Update every occurrence of the old version string:

| File | Locations |
|---|---|
| `README.md` | Status line: `> **Status:** vX.Y.Z` |
| `html/cleo/index.html` | Nav badge `.nav-version`, hero eyebrow, install verify output |
| `html/cleo/docs.html` | Nav badge `.nav-version`, install verify output, any status copy |
| `internal/cli/root.go` | `Version` constant without leading `v` |

Run to find stragglers:

```bash
rg "v<old-version>|<old-version>" README.md CHANGELOG.md html/cleo internal/cli/root.go
```

Avoid changing historical changelog headings except where release compare links or current status copy require it.

## 4. Canonical docs audit: `README.md`

`README.md` is the canonical docs surface linked by the landing page. Audit it against current code, not memory.

**Config schema and defaults:**

```bash
rg -n "type Config|type UI|type Tmux|type Sound|type Timeouts|type Pruning|Keybinds" internal/config
rg -n "Defaults_|Theme|Editor|SidebarWidth|EventLogLines|PanePreview|Agents" internal/config/defaults.go internal/config/schema.go
```

Update README config examples and tables for any additions, removals, renamed fields, or default changes. Recent important fields include `[ui].editor` and `[keybinds]`.

**Keybindings:**

```bash
rg -n "keybindActions|reservedKeys|specialKeys|resolveKeymap" internal/config/keymap.go
rg -n "KeyHint|Help|footer|keymap|Binding" internal/tui
```

Update the TUI key table and `[keybinds]` section. Current concepts to preserve:

- Defaults are configurable through `[keybinds]`.
- `enter`, `esc`, and `ctrl+c` are reserved hatches.
- Invalid keys are dropped with warnings; an action with no valid override falls back to defaults.
- Conflicts resolve by action order, first-wins.
- The help popup and footer derive labels from the resolved keymap.
- Popup-internal editing keys are not configurable.

**CLI and hooks:**

```bash
rg -n "new[A-Z].*Cmd|Use:|Short:|Long:|Flags\(" internal/cli
rg -n "Protocol|Install|Cleanup|doctor|hooks" internal
```

Update command examples, hook files, doctor behavior, install/cleanup text, and troubleshooting.

## 5. Landing page audit: `html/cleo/index.html`

The landing page is static and easy to drift. Review at least:

- Version refs: nav badge, hero eyebrow, install verify output.
- Docs links: current landing-page links point at the GitHub README `#documentation` anchor; troubleshooting should point at an existing repo path.
- Install flow: curl installer, Homebrew, hook setup, supported agents, uninstall commands.
- Terminal demo: project/session labels, states, footer keys, action names, event names.
- Feature cards: hooks support, sound behavior, local-first paths, key examples.
- Copy that names defaults: derive from `internal/config/defaults.go` and `internal/config/keymap.go`.

Check landing links before release:

```bash
rg -n "href=|docs\.html|troubleshooting|#documentation|releases|CHANGELOG" html/cleo/index.html
```

## 6. Secondary static docs audit: `html/cleo/docs.html`

This page still exists but is not the canonical docs link target. If it remains published, update it with the same user-facing truth as the README:

- Version badge and install verify output.
- Commands and flags.
- Hook install, cleanup, doctor, and supported-agent files.
- Config schema, including `[ui].editor` and `[keybinds]`.
- TUI key table: send is `m`, mute is `alt+m`, editor is `ctrl+g` or `e`, remove project is `D`, kill is `K` or `ctrl+k`.
- Validation and warning-popup behavior for keybind/config issues.

If the release intentionally retires or stops linking this page, note that in the PR/release notes and avoid leaving stale public links.

## 7. Global docs and link checks

Search for old commands, old versions, and stale links:

```bash
rg -n "cleo init|cleo hook |ctrl-k|ctrl\+k|alt\+m|ctrl\+g|docs\.html|v<old-version>" \
  README.md CHANGELOG.md docs html/cleo .pi/skills/release

rg -n "docs/troubleshooting\.md|troubleshooting" README.md docs html/cleo
```

If a path is linked, make sure the file exists.

## 8. Build, test, and smoke check

Run the normal test suite and a version smoke check:

```bash
go test ./...
make build
./bin/cleo --version
```

If HTML changed, open or preview the landing page and docs page enough to catch broken layout or copy mistakes.

## 9. Commit

```bash
git add CHANGELOG.md README.md html/cleo/docs.html html/cleo/index.html internal/cli/root.go
# add any other changed files
git commit -m "chore: bump version to vX.Y.Z, update docs and changelog

- Version: <old> to <new> in cli, landing page, docs, README
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
