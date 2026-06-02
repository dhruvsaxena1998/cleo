# Commands

The full `cleo` subcommand reference. For configuration options, see [Configuration](configuration.md); for how hook events drive session state, see [Hooks & State](hooks.md).

## `cleo`

Launches the TUI dashboard.

```bash
cleo
```

See [Quick Start › TUI Dashboard](quickstart.md#tui-dashboard) for the dashboard keys.

## `cleo hooks init`

Installs Cleo hook commands into supported agent config files and extracts bundled sound assets.

```bash
cleo hooks init
cleo hooks init --agents claude,codex
cleo hooks init --force --agents claude,codex,pi,opencode
```

Options:

| Option | Meaning |
| --- | --- |
| `--agents <list>` | Comma-separated agents to install (`claude`, `codex`, `opencode`, `pi`); skips interactive prompts |
| `--force` | Overwrite conflicting hook entries |

Installed files:

| Agent | Files |
| --- | --- |
| Claude Code | `~/.claude/settings.json` |
| Codex | `~/.codex/hooks.json`, `~/.codex/config.toml` |
| Pi | `~/.pi/agent/extensions/cleo.ts` |
| OpenCode | `~/.config/opencode/plugins/cleo.ts` |

For Codex, `cleo hooks init` also ensures `[features].hooks = true` exists in `~/.codex/config.toml`. After installing Codex hooks, restart open Codex sessions and run `/hooks` in Codex to approve the Cleo hook entries if they appear under review.

## `cleo doctor`

Checks whether Cleo hooks look correctly installed and whether hook events have recently resolved to a Cleo session.

```bash
cleo doctor
cleo doctor --quiet
```

Options:

| Option | Meaning |
| --- | --- |
| `--quiet` | Only print failures and non-empty diagnostic sections; exits non-zero when failures are found |

This command checks:

- Claude Code hook entries.
- Codex hook feature flag.
- Codex hook entries.
- Pi extension status.
- OpenCode plugin status.
- Recent hook trace activity.

Codex keeps hook approval state internally, so `doctor` can verify files but cannot prove that Codex has approved every hook. Use `/hooks` inside Codex for that final approval state.

## `cleo hooks cleanup`

Removes Cleo hook commands from supported agent config files.

```bash
cleo hooks cleanup
cleo hooks cleanup --agents claude,codex
```

Options:

| Option | Meaning |
| --- | --- |
| `--agents <list>` | Comma-separated agents to clean up (`claude`, `codex`, `opencode`, `pi`); skips interactive prompts |

`cleanup` removes Cleo entries from supported agent hook files. It leaves `~/.codex/config.toml` `[features].hooks` unchanged because other Codex hooks may depend on that flag.

## `cleo add [path]`

Registers a project.

```bash
cleo add
cleo add ~/Dev/myapp
```

If no path is provided, Cleo registers the current working directory. Project IDs are slugified from the directory name and deduplicated if needed.

## `cleo rm <project>`

Unregisters a project.

```bash
cleo rm myapp
cleo rm myapp --yes
cleo rm myapp --force --yes
```

Options:

| Option | Meaning |
| --- | --- |
| `--force` | Remove the project even if it still has active sessions in Cleo state |
| `--yes`, `-y` | Skip confirmation |

Running tmux sessions keep running unless `--force` best-effort kills active sessions and removes their Cleo state records. This removes the project from Cleo's project registry; it does not delete your project directory.

## `cleo run <agent>`

Starts an agent session in tmux.

```bash
cleo run claude
cleo run codex --name refactor-auth
cleo run claude --cwd ~/Dev/myapp --name fix-tests --no-attach
cleo run codex --yes
```

Options:

| Option | Meaning |
| --- | --- |
| `--name <name>` | Human-friendly session name. Cleo slugifies and deduplicates it. |
| `--cwd <path>` | Start from this working directory instead of the current directory. |
| `--yes` | Skip confirmation when auto-registering a new project. |
| `--no-attach` | Spawn the session but do not attach to it immediately. |

Session IDs follow this shape:

```text
cleo-<project-id>-<agent>-<session-name>
```

If you do not pass `--name`, Cleo assigns a Docker-style generated label such as `brave-curie` or `steady-turing`.

## `cleo ls`

Lists registered projects and known sessions.

```bash
cleo ls
cleo ls --json
```

Options:

| Option | Meaning |
| --- | --- |
| `--json` | Output project/session rows as JSON |

The table output includes project ID, agent, session name, state, full session ID, and age.

## `cleo attach <session-id>`

Attaches to an existing tmux session.

```bash
cleo attach cleo-myapp-claude-fix-auth-bug
```

Detach with the configured tmux detach key, usually `Ctrl-b d`.

## `cleo rename <session-id> <new-name>`

Renames the Cleo-side session label. The underlying tmux session ID is **not** changed, so attach commands and hook attribution keep working unchanged.

```bash
cleo rename cleo-myapp-claude-fix-auth-bug fix-auth-take-2
```

The new name is slugified the same way `--name` is on `cleo run`. To rename interactively in the dashboard, select a session and press `r`.

## `cleo kill <session-id>`

Kills a running tmux session and removes it from Cleo state.

```bash
cleo kill cleo-myapp-codex-1
cleo kill cleo-myapp-codex-1 --yes
```

Options:

| Option | Meaning |
| --- | --- |
| `--yes` | Skip confirmation |

## `cleo prune [project]`

Removes finished sessions from active Cleo state and archives their event logs.

```bash
cleo prune
cleo prune myapp
cleo prune myapp --keep 10
cleo prune --all --dry-run
cleo prune --yes
```

Options:

| Option | Meaning |
| --- | --- |
| `--keep <n>` | Keep the newest `n` finished sessions per project. Defaults to `pruning.keep_default`. |
| `--all` | Consider sessions across all projects. |
| `--dry-run` | Print sessions that would be pruned without changing state. |
| `--yes` | Skip confirmation. |

Finished session states are `completed`, `error`, and `dead`.
