---
status: accepted
---

# tmux access goes through one consumer-side interface

The Session lifecycle drives tmux through a single interface, `sessionlifecycle.Tmux`, that names every method it calls (`NewSession`, `HasSession`, `BindDetachKey`, `InstallFocusHooks`, `Kill`). The interface is declared by the consumer (sessionlifecycle), not exported from the producer (the `tmux` package), and is not split into role-segregated interfaces (Launcher / Inspector / FocusInstaller / Killer).

## Why

The lifecycle previously declared `TmuxLauncher` (just `NewSession`) and recovered the other four methods at call sites via anonymous type assertions (`l.tmux.(interface{ HasSession(...) })`). That made the declared interface lie about its real dependencies, and — worse — made a missing method silently no-op: `verifySessionAlive` returned "alive" for any launcher without `HasSession`. A single honest interface makes the dependency explicit, lets the compiler enforce that every adapter (the production `tmux.Client` and the test fake) satisfies the whole contract, and removes the silent-no-op trapdoor.

## Considered and rejected

- **Producer-side interface (`tmux.Adapter`)** — exporting the interface from the `tmux` package drifts toward a fat catch-all and inverts the Go idiom of consumers declaring their own needs.
- **Role-segregated interfaces** — splitting into Launcher/Inspector/FocusInstaller/Killer re-introduces the fragmentation this change removes and forces the lifecycle to compose several seams for no gain at five total methods.

Do not re-suggest either in future architecture reviews without new evidence (e.g. a second non-tmux adapter that genuinely needs only a subset).

## Consequences

`cli.TmuxClient` gains `BindDetachKey` and `InstallFocusHooks` so `Ctx.Tmux` remains assignable to `sessionlifecycle.Tmux` by structural subtyping; the now-redundant `cli.TmuxFocusInstaller` interface is deleted. The narrow `reconcile.TmuxLs` interface (only `LsPrefix`) is a separate, correctly-scoped consumer and is left untouched.
