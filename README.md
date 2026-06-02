# cleo

Terminal session manager for AI coding agents.

> **Status:** v0.1.2 — current stable release. Config and CLI surface may still change before v1.0. Bug reports and feedback welcome.

Cleo lets you run Claude Code, Codex, opencode, pi, or any other terminal-based agent in named tmux sessions, then watch and manage those sessions from one TUI dashboard. Sessions live in tmux, so you can close Cleo, reopen it later, and keep long-running agent work intact.

In v0.1, hook-based lifecycle tracking is implemented for **Claude Code**, **Codex**, **Pi**, and **OpenCode** when their hook integrations are installed with `cleo hooks init`. Other terminal agents can still be managed through tmux, with less detailed lifecycle tracking.

## Why it exists

Running several agents at once means juggling several tmux sessions by hand — remembering which session is which, which one has stopped and is waiting on you, and which tmux incantation reattaches you to it. Cleo gives that fleet a single dashboard: every project and session in one place, each one's live state at a glance, and a sound when a session needs your attention, so you can let agents run in the background without losing track of them.

## What Cleo Does

- Registers local projects you want to manage.
- Spawns agent sessions in tmux with stable session IDs.
- Shows all registered projects and sessions in a terminal dashboard.
- Tracks agent state through supported agent hook events.
- Plays local sounds for important transitions, such as session start, completion, errors, and requests for input.
- Keeps per-session event logs and archives them when sessions are pruned.
- Lets you attach, view, rename, kill, and clean up sessions without remembering tmux commands.

Cleo is intentionally local-first. It stores its state in your config directory, runs agents on your machine, and does not require a service process.

## Install

```bash
curl -fsSL https://github.com/dhruvsaxena1998/cleo/raw/main/scripts/install.sh | sh
```

Homebrew and `go install` options, requirements, upgrade, and uninstall are in the [Installation guide](docs/installation.md).

## Documentation

Full user documentation lives under [`docs/`](docs/):

- **[Installation](docs/installation.md)** — requirements, install, verify, upgrade, uninstall.
- **[Quick Start](docs/quickstart.md)** — first run, core workflow, the TUI dashboard and its keys.
- **[Commands](docs/commands.md)** — full `cleo` subcommand reference.
- **[Configuration](docs/configuration.md)** — `config.toml` reference (`[tmux]`, `[sound]`, `[agents]`, `[ui]`, `[keybinds]`), recipes, and environment variables.
- **[Hooks & State](docs/hooks.md)** — hook events, session states, and the files Cleo manages.
- **[Troubleshooting](docs/troubleshooting.md)** — recovery recipes for common failure modes.
- **[Aliases](docs/aliases.md)** — map a one-word alias straight to a running agent session.

## Development

```bash
make build         # go build -o bin/cleo ./cmd/cleo
make test          # go test ./...
make lint          # go vet ./...
make run           # build and launch ./bin/cleo
./scripts/smoke.sh # end-to-end manual smoke; requires claude CLI and tmux
```

Useful design notes live in:

- `docs/superpowers/specs/2026-05-07-cleo-design.md`
- `docs/superpowers/plans/2026-05-07-cleo-implementation.md`

## License

MIT. See [LICENSE](LICENSE).
