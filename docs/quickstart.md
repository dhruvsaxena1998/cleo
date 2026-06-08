# Quick Start

Get from a fresh install to a managed agent session and the dashboard. If you haven't installed Cleo yet, see [Installation](installation.md).

```bash
# Install Cleo hook entries for supported agents.
cleo hooks init

# Register the project you want Cleo to manage.
cd ~/Dev/myapp
cleo add

# Start a Claude Code session in that project.
cleo run claude --name fix-auth-bug

# Open the dashboard.
cleo
```

When Cleo attaches you to a tmux session, detach back to the dashboard with your configured tmux detach key. The default is `Ctrl-b d`.

## Core Workflow

1. Run `cleo hooks init` once per machine to install hooks for supported agents.
2. Run `cleo add [path]` for each project you want visible in the dashboard.
3. Start sessions with `cleo run <agent> --name <task-name>`.
4. Use `cleo` to monitor session states and attach to work that needs attention.
5. Run `cleo prune` periodically to remove finished sessions from the active state file while keeping archived event logs.

Cleo auto-registers a project during `cleo run` if the working directory is not known yet. Use `--yes` to skip that confirmation.

For the full subcommand reference, see [Commands](commands.md). For a one-word "spawn and attach" shortcut, see [Aliases](aliases.md).

## TUI Dashboard

Run:

```bash
cleo
```

The dashboard shows registered projects, their sessions, current state, and a preview/event pane. It reconciles state with tmux so sessions whose tmux processes disappeared can be marked `dead`. It also supports mouse interaction (clicking rows to select, clicking an already-selected row to attach, clicking headers to toggle project collapse, and wheel scrolling).

### Keys

| Key | Action |
| --- | --- |
| `up` / `k` | Move up |
| `down` / `j` | Move down |
| `space` | Expand or collapse a project |
| `enter` | Attach to the selected session |
| `ctrl+g` or `e` | Open the selected Project in your editor |
| `n` | Start a new session |
| `v` | View a selected session without attaching |
| `m` | Send text to the selected session |
| `r` | Rename a session |
| `K` or `ctrl+k` | Kill the selected session |
| `/` | Filter projects and sessions |
| `P` | Prune finished sessions for the focused project |
| `D` | Remove the focused project |
| `alt+m` | Toggle sound for the running Cleo process |
| `,` | Open the in-app settings editor |
| `?` | Show help |
| `esc` | Cancel the current popup/filter mode |
| `q` | Quit the dashboard |

The footer changes based on the selected row, so the safest source of truth while using the app is the action list shown at the bottom of the TUI.

These bindings are configurable — rebind any action via the [`[keybinds]`](configuration.md#keybinds) table in `config.toml`. The `esc`, `enter`, and `ctrl+c` keys are reserved hatches (cancel, confirm/attach, quit) and cannot be reassigned.
