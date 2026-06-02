---
status: accepted
---

# GitHub-rendered markdown is the single source of truth for documentation

Cleo's user documentation lived twice: as prose in `README.md` and as a hand-built, separately-styled `html/cleo/docs.html` that re-rendered the same install/quick-start/commands/configuration/hooks/troubleshooting content. We are deleting `docs.html` and making markdown on GitHub the single source: the seven user-facing reference topics become focused pages under `docs/` (`installation`, `quickstart`, `commands`, `configuration`, `hooks`, `troubleshooting`, `aliases`), `README.md` becomes a slim hub (what it does / why it exists / the problem it solves, plus a page-by-page deep-link index), and the marketing landing page (`html/cleo/index.html`) stays hand-crafted static HTML whose "docs" links point into GitHub. No static-site generator, no docs framework, no front-end build step.

## Why

The duplication was a recurring tax, not a one-time cost: the release skill (`.pi/skills/release/`) listed `docs.html` as a place to hand-sync version numbers, the config schema, the command reference, and keybinding tables on *every* release, so the two copies drifted whenever someone forgot a step. Markdown rendered by GitHub removes the second copy entirely and the toolchain that would maintain it — which matters in a pure-Go repo that otherwise has zero front-end build.

The accepted cost is deliberate: docs now leave Cleo's own domain and lose the bespoke papyrus/brass styling — a reader who clicks "docs" on the polished landing page lands on github.com's generic markdown rendering. For a CLI tool whose audience already lives on GitHub, that is an acceptable trade for true single-source docs and the deleted release step. A small, bounded overlap remains by design — the curl install one-liner and the seven-item doc index appear in both `README.md` and the docs pages — but these are short and change ~never, unlike the config schema that drove the original drift.

## Considered and rejected

- **Docusaurus / VitePress (an opinionated docs framework).** Gives markdown SSOT, auto-generated nav, built-in search, and doc versioning out of the box. Rejected because it imposes a React/Node `node_modules` build on an otherwise pure-Go repo and would force abandoning or heavily reworking the distinctive hand-tuned landing-page design (oklch palette, Iowan serif, hand-tuned light/dark) to fit the framework's theme.
- **Astro / 11ty (flexible SSG feeding markdown into our own templates).** Would preserve the bespoke design *and* give markdown SSOT, but adds a build step and glue code to maintain for a benefit — on-domain, on-brand docs — we decided was not worth it at this stage.
- **Keeping `docs.html` but generating it from the markdown at build time.** Still introduces a build step and a generator to maintain, for the sole gain of on-domain styling.

## Consequences

- `html/cleo/docs.html` is deleted; the four `index.html` links that referenced it repoint to GitHub (`#documentation` anchor ×3, `docs/troubleshooting.md` ×1).
- The release skill drops every `docs.html` sync step; version references now live only in `README.md` and `index.html`.
- Browsing the `docs/` folder on GitHub shows a plain file listing (there is no `docs/README.md` index by choice — the root `README.md` is the single index); users reach pages via the README's deep links, not by browsing the folder.
- Contributor/internal docs (`docs/glossary.md`, ADRs, `CONTEXT.md`, feature-ideas) are unaffected and stay off the user "docs" path.
- Docs are served from `main`, so the published docs always reflect the latest committed state rather than the last release tag.
