---
name: release
description: Bump cleo version, update docs, changelog, and landing page, then tag and push a release. Use when user says "release", "new version", "tag", "ship", "bump version", or after merging features for a release.
---

# Release cleo

## Quick start

```bash
# After all changes are committed:
git tag -a vX.Y.Z -m "vX.Y.Z: <summary>" && git push origin vX.Y.Z
```

GitHub Actions (`.github/workflows/release.yml`) runs goreleaser on pushed tags, building binaries and updating the Homebrew tap.

## Process

See [CHECKLIST.md](CHECKLIST.md) for the full step-by-step. High-level order:

1. **Version** — bump in `internal/cli/root.go`
2. **Changelog** — add `[X.Y.Z]` section to `CHANGELOG.md`
3. **Docs** — update version refs in `README.md`, `html/cleo/index.html`, `html/cleo/docs.html`
4. **Config/docs surface** — if config schema, CLI surface, keybinds, or defaults changed, update README + docs.html + landing page
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
| `html/cleo/docs.html` | Version (×n), config schema, command docs |
