# cleo

Terminal session manager for AI coding agents. Manages multiple Claude Code, Codex, and other agent sessions in tmux with a TUI dashboard. Sessions persist in tmux, so cleo can be closed and reopened freely without disrupting any running work.

## Install

```bash
go install github.com/dhruvsaxena1998/cleo/cmd/cleo@latest
```

Requires Go 1.22+ and tmux 3.0+ at runtime.

## Quick start

```bash
cleo init                                # one-time: install hooks into ~/.claude/settings.json
cd ~/Dev/myapp && cleo add               # register the project
cleo run claude --name fix-auth-bug      # spawn a claude session in this project
cleo                                     # open the TUI dashboard
```

In the TUI: `↑/↓` navigate · `space` expand/collapse · `n` new agent · `v` view · `↵` attach · `k` kill · `/` filter · `m` mute · `q` quit.

When attached to a tmux session, detach back with `Ctrl-b d`.

## Commands

| Command                              | Purpose                                                      |
| ------------------------------------ | ------------------------------------------------------------ |
| `cleo`                               | Launch TUI                                                   |
| `cleo init`                          | One-time hook installer                                      |
| `cleo add [path]`                    | Register a project (default: cwd)                            |
| `cleo rm <project>`                  | Unregister (running sessions keep running)                   |
| `cleo run <agent> [--name N] [--yes]`| Spawn an agent in the current project                        |
| `cleo ls`                            | List projects + sessions                                     |
| `cleo attach <session-id>`           | Attach to a tmux session                                     |
| `cleo kill <session-id> [--yes]`     | Kill a running session                                       |
| `cleo prune [project] [--keep N]`    | Remove finished sessions; archives event logs                |

## Configuration

`~/.config/cleo/config.toml` is created on first run with sensible defaults. Edit to add agents, change brand colors, mute sounds, or tune retention. See the `[agents.*]`, `[sound]`, `[ui]`, and `[retention]` sections.

## Hooks

cleo observes agent state via Claude Code's and Codex's native hook systems. `cleo init` installs the hook entries that invoke `cleo hook claude <event>` (and equivalent for codex when configured) on every state-changing event. The shim updates `~/.config/cleo/state.json` and plays sounds for state transitions.

For agents without hook support (opencode, pi in v0.1), sessions show perpetual `running` until the tmux session ends.

## Troubleshooting

- **`cleo init` says "conflict"** — your `~/.claude/settings.json` already has different hook entries. Resolve manually, then re-run.
- **No sound on macOS** — verify `afplay` is on PATH (`which afplay`). Real WAV assets must be present in `~/.config/cleo/sounds/` (cleo init extracts the bundled defaults).
- **Cleo binary moved** — re-run `cleo init` so the hook configs point at the new path.
- **`cleo ls` shows session as `dead`** — the underlying tmux session went away. Run `cleo prune` to clean up.

## Development

```bash
make build         # produces bin/cleo
make test          # go test ./...
make lint          # go vet ./...
./scripts/smoke.sh # end-to-end manual smoke (requires claude CLI + tmux)
```

Project layout follows the design at `docs/superpowers/specs/2026-05-07-cleo-design.md`. Implementation plan at `docs/superpowers/plans/2026-05-07-cleo-implementation.md`.

## License

TBD.
