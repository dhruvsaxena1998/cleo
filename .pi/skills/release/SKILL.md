---
name: release
description: Bump cleo version, update changelog and version references, then tag and push a release. Use when user says "release", "new version", "tag", "ship", "bump version", or after merging features for a release.
---

# Release cleo

## Quick start

```bash
# After all changes are committed:
git tag -a vX.Y.Z -m "vX.Y.Z: <summary>" && git push origin vX.Y.Z
```

GitHub Actions (`.github/workflows/release.yml`) runs goreleaser on pushed tags, building binaries and updating the Homebrew tap.

## Process

**Step 0 — determine version.** Always start by finding the latest tag and asking the user what the next version should be:

```bash
git tag --sort=-version:refname | head -3
```

Present the latest tag (e.g., `v0.1.2`) and ask: *"What should the next version be?"* Suggest the likely semver bump (patch/minor). Do not proceed until the user confirms a version.

See [CHECKLIST.md](CHECKLIST.md) for the full step-by-step. High-level order:

1. **Version** — bump in `internal/cli/root.go`
2. **Changelog** — add `[X.Y.Z]` section to `CHANGELOG.md`
3. **Version refs** — update version references in `README.md` and `html/cleo/index.html`
4. **Landing-page surface** — if config schema, CLI surface, keybinds, or defaults changed, update the bits mirrored on the landing page (`html/cleo/index.html`). The `docs/` markdown pages are the source of truth and are edited as part of the feature work, so they need no release-time re-sync.
5. **Commit** — `"chore: bump version to vX.Y.Z"`
6. **Tag** — `git tag -a vX.Y.Z` with release notes summary
7. **Push** — tag triggers goreleaser

## Key files

| File | What to update |
|---|---|
| `internal/cli/root.go` | `Version` constant |
| `CHANGELOG.md` | New `[X.Y.Z]` section |
| `README.md` | Status line, config defaults, CLI changes |
| `html/cleo/index.html` | Version (×3), keybinds, features, install commands |
