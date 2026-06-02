# Release Checklist

## 0. Determine version

Find the latest tag:

```bash
git tag --sort=-version:refname | head -3
```

**Ask the user** what version to release next. Print the latest tag and suggest the likely semver bump:
- Patch (`v0.1.2` → `v0.1.3`) for bug fixes only
- Minor (`v0.1.2` → `v0.2.0`) for new features
- Major (`v0.1.2` → `v1.0.0`) for breaking changes

Do not proceed until the user confirms a specific version number.

## 1. Bump version

Edit `internal/cli/root.go`:

```go
var Version = "X.Y.Z"
```

## 2. Changelog

Edit `CHANGELOG.md`. Add a new `[X.Y.Z] - YYYY-MM-DD` section above the previous release. Pull features/fixes from `git log <last-tag>..HEAD --format="%h %s%n%b"`.

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

## 3. Version references

Update every occurrence of the old version string:

| File | Locations |
|---|---|
| `README.md` | Status line: `> **Status:** vX.Y.Z` |
| `html/cleo/index.html` | Nav badge `.nav-version`, hero eyebrow, install section version output |

Run to find stragglers:

```bash
grep -rn "v<old-version>" --include="*.html" --include="*.go" --include="*.md" \
  | grep -v ".git/" | grep -v "superpowers/" | grep -v "CHANGELOG"
```

## 4. Config & CLI surface audit

The `docs/` markdown pages are the source of truth for the config schema, command, and keybind reference, and are edited as part of the feature work that changed them — they need no release-time re-sync. At release time, audit only the hand-maintained landing page (`html/cleo/index.html`) for drift against those changes:

**Config schema changes** (fields added/removed/renamed, defaults changed):
- `html/cleo/index.html` — config example, feature copy (source of truth: `docs/configuration.md`)

**CLI surface changes** (new/renamed/removed commands or flags):
- `html/cleo/index.html` — install/usage snippets (source of truth: `docs/commands.md`, `docs/quickstart.md`)

**Keybind changes** (new/removed/renamed keybindings):
- `html/cleo/index.html` — TUI demo footer, features section (source of truth: `docs/configuration.md`)

**Landing page** — review `html/cleo/index.html` for:
- Install instructions (e.g., `cleo init` → `cleo hooks init`)
- TUI demo footer keybinds
- Feature descriptions
- Terminal demo content (session states, agent labels)

## 5. Commit

```bash
git add CHANGELOG.md README.md html/cleo/index.html internal/cli/root.go
# add any other changed files
git commit -m "chore: bump version to vX.Y.Z, update docs and changelog

- Version: <old> → <new> in cli, landing page, README
- CHANGELOG: add vX.Y.Z section
- <other changes summarized>"
git push
```

## 6. Tag

```bash
git tag -a vX.Y.Z -m "vX.Y.Z: <brief summary of headliners>"
git push origin vX.Y.Z
```

GitHub Actions (`release.yml`) triggers on `v*` tags and runs goreleaser:
- Builds binaries for macOS (arm64, amd64) and Linux (arm64, amd64)
- Creates GitHub Release with artifacts
- Updates Homebrew formula in `dhruvsaxena1998/homebrew-tap`

Verify the release at: `https://github.com/dhruvsaxena1998/cleo/releases`
