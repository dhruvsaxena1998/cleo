# Release Checklist

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
| `html/cleo/docs.html` | Nav badge `.nav-version`, install section version output |

Run to find stragglers:

```bash
grep -rn "v<old-version>" --include="*.html" --include="*.go" --include="*.md" \
  | grep -v ".git/" | grep -v "superpowers/" | grep -v "CHANGELOG"
```

## 4. Config & CLI surface audit

If any of these changed since the last release, update the corresponding docs:

**Config schema changes** (fields added/removed/renamed, defaults changed):
- `README.md` — config reference table and example
- `html/cleo/docs.html` — `[ui]` section, config example, `cmd-table`

**CLI surface changes** (new/renamed/removed commands or flags):
- `README.md` — quick start and command examples
- `html/cleo/docs.html` — command reference section

**Keybind changes** (new/removed/renamed keybindings):
- `html/cleo/index.html` — TUI demo footer, features section
- `html/cleo/docs.html` — keybinding table (if present)

**Landing page** — review `html/cleo/index.html` for:
- Install instructions (e.g., `cleo init` → `cleo hooks init`)
- TUI demo footer keybinds
- Feature descriptions
- Terminal demo content (session states, agent labels)

## 5. Commit

```bash
git add CHANGELOG.md README.md html/cleo/docs.html html/cleo/index.html internal/cli/root.go
# add any other changed files
git commit -m "chore: bump version to vX.Y.Z, update docs and changelog

- Version: <old> → <new> in cli, landing page, docs, README
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
