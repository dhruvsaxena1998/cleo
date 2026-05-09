# cleo

Terminal session manager for AI coding agents.

> **Status:** v0.1 alpha. Expect rough edges and breaking changes between minor releases. Bug reports and feedback welcome.

Cleo lets you run Claude Code, Codex, opencode, pi, or any other terminal-based agent in named tmux sessions, then watch and manage those sessions from one TUI dashboard. Sessions live in tmux, so you can close Cleo, reopen it later, and keep long-running agent work intact.

In v0.1, hook-based lifecycle tracking is implemented for **Claude Code** and **Codex**. **opencode** and **pi** are supported as managed tmux sessions (you can spawn, attach, kill, prune them) but Cleo cannot observe their fine-grained state — they remain `running` until the underlying tmux session ends.

## What Cleo Does

- Registers local projects you want to manage.
- Spawns agent sessions in tmux with stable session IDs.
- Shows all registered projects and sessions in a terminal dashboard.
- Tracks agent state through Claude Code and Codex hook events.
- Plays local sounds for important transitions, such as session start, completion, errors, and requests for input.
- Keeps per-session event logs and archives them when sessions are pruned.
- Lets you attach, view, rename, kill, and clean up sessions without remembering tmux commands.

Cleo is intentionally local-first. It stores its state in your config directory, runs agents on your machine, and does not require a service process.

## Requirements

- Go `1.25.5` (only required if building from source; prebuilt releases are available via Homebrew or the Releases page).
- `tmux` `3.0+` at runtime.
- The agent CLIs you want to use, such as `claude`, `codex`, `opencode`, or `pi`.
- Sound playback uses `afplay` on macOS and the first of `paplay`, `aplay`, or `play` available on Linux. Windows is not supported in v0.1.

## Install

```bash
go install github.com/dhruvsaxena1998/cleo/cmd/cleo@latest
```

For local development from this repository:

```bash
make build
./bin/cleo --version
```

## Quick Start

```bash
# Install Cleo hook entries for supported agents.
cleo init

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

1. Run `cleo init` once per machine to install hooks for Claude Code and Codex.
2. Run `cleo add [path]` for each project you want visible in the dashboard.
3. Start sessions with `cleo run <agent> --name <task-name>`.
4. Use `cleo` to monitor session states and attach to work that needs attention.
5. Run `cleo prune` periodically to remove finished sessions from the active state file while keeping archived event logs.

Cleo auto-registers a project during `cleo run` if the working directory is not known yet. Use `--yes` to skip that confirmation.

## TUI Dashboard

Run:

```bash
cleo
```

The dashboard shows registered projects, their sessions, current state, and a preview/event pane. It reconciles state with tmux so sessions whose tmux processes disappeared can be marked `dead`.

### Keys

| Key | Action |
| --- | --- |
| `up` / `k` | Move up |
| `down` / `j` | Move down |
| `space` | Expand or collapse a project |
| `enter` | Attach to the selected session |
| `n` | Start a new session |
| `v` | View a selected session without attaching |
| `r` | Rename a session |
| `K` or `ctrl+k` | Kill or remove the selected session |
| `/` | Filter projects and sessions |
| `m` | Toggle sound for the running Cleo process |
| `?` | Show help |
| `esc` | Cancel the current popup/filter mode |
| `q` | Quit the dashboard |

The footer changes based on the selected row, so the safest source of truth while using the app is the action list shown at the bottom of the TUI.

## Commands

### `cleo`

Launches the TUI dashboard.

```bash
cleo
```

### `cleo init`

Installs Cleo hook commands into supported agent config files and extracts bundled sound assets.

```bash
cleo init
cleo init --yes
cleo init --force
```

Options:

| Option | Meaning |
| --- | --- |
| `--yes`, `-y` | Install all supported hook systems without prompting |
| `--force` | Overwrite conflicting hook entries |

Installed files:

| Agent | Files |
| --- | --- |
| Claude Code | `~/.claude/settings.json` |
| Codex | `~/.codex/hooks.json`, `~/.codex/config.toml` |

For Codex, `cleo init` also ensures `[features].hooks = true` exists in `~/.codex/config.toml`. After installing Codex hooks, restart open Codex sessions and run `/hooks` in Codex to approve the Cleo hook entries if they appear under review.

### `cleo doctor`

Checks whether Cleo hooks look correctly installed and whether hook events have recently resolved to a Cleo session.

```bash
cleo doctor
```

This command checks:

- Claude Code hook entries.
- Codex hook feature flag.
- Codex hook entries.
- Recent Claude and Codex hook trace activity.

Codex keeps hook approval state internally, so `doctor` can verify files but cannot prove that Codex has approved every hook. Use `/hooks` inside Codex for that final approval state.

### `cleo cleanup`

Removes Cleo hook commands from supported agent config files.

```bash
cleo cleanup
cleo cleanup --yes
cleo uninstall --yes
```

`cleanup` removes Cleo entries from Claude Code and Codex hook files. It leaves `~/.codex/config.toml` `[features].hooks` unchanged because other Codex hooks may depend on that flag.

### `cleo add [path]`

Registers a project.

```bash
cleo add
cleo add ~/Dev/myapp
```

If no path is provided, Cleo registers the current working directory. Project IDs are slugified from the directory name and deduplicated if needed.

### `cleo rm <project>`

Unregisters a project.

```bash
cleo rm myapp
```

Running tmux sessions keep running. This removes the project from Cleo's project registry; it does not delete your project directory.

### `cleo run <agent>`

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

### `cleo ls`

Lists registered projects and known sessions.

```bash
cleo ls
```

The output includes project ID, agent, session name, state, and full session ID.

### `cleo attach <session-id>`

Attaches to an existing tmux session.

```bash
cleo attach cleo-myapp-claude-fix-auth-bug
```

Detach with the configured tmux detach key, usually `Ctrl-b d`.

### `cleo kill <session-id>`

Kills a running tmux session and removes it from Cleo state.

```bash
cleo kill cleo-myapp-codex-1
cleo kill cleo-myapp-codex-1 --yes
```

Options:

| Option | Meaning |
| --- | --- |
| `--yes` | Skip confirmation |

### `cleo prune [project]`

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
| `--keep <n>` | Keep the newest `n` finished sessions per project. Defaults to `retention.prune_keep_default`. |
| `--all` | Consider sessions across all projects. |
| `--dry-run` | Print sessions that would be pruned without changing state. |
| `--yes` | Skip confirmation. |

Finished session states are `completed`, `error`, and `dead`.

## Configuration

Cleo reads and writes:

```text
~/.config/cleo/config.toml
```

If `XDG_CONFIG_HOME` is set, Cleo uses:

```text
$XDG_CONFIG_HOME/cleo/config.toml
```

The config file is created on first run with defaults. You can edit it by hand.

### Default Config Shape

```toml
[defaults]
detach_key = "C-b d"
default_agent = "claude"

[sound]
enabled = true
volume = 0.7

[sound.events]
session_start = "start.wav"
needs_input = "attention.wav"
session_idle = "done.wav"
session_completed = "done.wav"
session_error = "error.wav"

[sound.event_enabled]
session_start = true
needs_input = true
session_idle = true
session_completed = true
session_error = true

[agents.claude]
command = "claude"
label = "cl"
color = "#CC785C"
hooks = "claude"

[agents.codex]
command = "codex"
label = "cx"
color = "#10A37F"
hooks = "codex"

[agents.opencode]
command = "opencode"
label = "oc"
color = "#FF6B35"
hooks = "none"

[agents.pi]
command = "pi"
label = "pi"
color = "#7C3AED"
hooks = "none"

[ui]
show_pane_preview = true
pane_preview_lines = 30
pane_preview_interval = "1.5s"
event_log_lines = 200
sidebar_width = 32

[retention]
hint_threshold = 6
prune_keep_default = 5
idle_to_completed_timeout = "10m"
spawning_timeout = "30s"
```

Durations use Go duration strings such as `"500ms"`, `"1.5s"`, `"30s"`, `"10m"`, or `"1h"`.

### `[defaults]`

| Key | Default | Meaning |
| --- | --- | --- |
| `detach_key` | `"C-b d"` | Tmux detach key Cleo tries to bind for spawned sessions. |
| `default_agent` | `"claude"` | Default agent name for flows that need one. |

### `[sound]`

| Key | Default | Meaning |
| --- | --- | --- |
| `enabled` | `true` | Enables sound playback for hook-triggered state transitions. |
| `volume` | `0.7` | Playback volume passed to Cleo's sound player. |

### `[sound.events]`

Maps Cleo event names to audio files. Relative paths are resolved under:

```text
~/.config/cleo/sounds/
```

Absolute paths are used as-is.

Supported event keys:

| Event | When it plays |
| --- | --- |
| `session_start` | A hook reports that a session started. |
| `needs_input` | The agent requests user input or tool permission. |
| `session_idle` | The agent stops and is considered idle. |
| `session_completed` | The agent reports session end. |
| `session_error` | Cleo records an error state. |

`cleo init` extracts bundled default WAV files into the sounds directory.

### `[sound.event_enabled]`

Turns individual sound events on or off while keeping their file mappings configured.

| Event | Default |
| --- | --- |
| `session_start` | `true` |
| `needs_input` | `true` |
| `session_idle` | `true` |
| `session_completed` | `true` |
| `session_error` | `true` |

For example, to keep attention and error sounds but disable startup and completion sounds:

```toml
[sound.event_enabled]
session_start = false
needs_input = true
session_idle = true
session_completed = false
session_error = true
```

Missing event toggles default to enabled for backward compatibility. You can also disable all sounds at once with `sound.enabled = false`.

### Focus-aware sound suppression

Cleo suppresses session sounds while that exact tmux session is focused. This prevents duplicate attention sounds when you are already attached to the agent and watching it work.

When Cleo starts or attaches to a tmux session, it enables tmux `focus-events` and installs tmux client focus hooks. Those hooks update Cleo's local focus state on attach, detach, focus-in, and focus-out. If you switch from the terminal to another app such as VS Code or Chrome, tmux can emit focus-out and Cleo will resume playing notification sounds for that session.

This is best-effort because terminal focus reporting depends on tmux and terminal emulator support. If Cleo cannot determine focus, it plays sounds rather than risk hiding an alert.

### `[agents.<name>]`

Defines agents you can pass to `cleo run <agent>`.

| Key | Meaning |
| --- | --- |
| `command` | Executable command Cleo starts inside tmux. |
| `label` | Short label shown in compact UI surfaces. |
| `color` | Hex color used by the TUI for that agent. |
| `hooks` | Hook protocol: `"claude"`, `"codex"`, or `"none"`. |

Example custom agent:

```toml
[agents.myagent]
command = "myagent"
label = "ma"
color = "#3B82F6"
hooks = "none"
```

After this, run:

```bash
cleo run myagent --name investigate-cache
```

Agents with `hooks = "none"` can still be spawned and managed through tmux, but Cleo cannot observe their detailed lifecycle. They generally stay `running` until the tmux session ends or Cleo reconciles them as `dead`.

### `[ui]`

| Key | Default | Meaning |
| --- | --- | --- |
| `show_pane_preview` | `true` | Show tmux pane output previews in the dashboard. |
| `pane_preview_lines` | `30` | Number of tmux pane lines to capture for preview. |
| `pane_preview_interval` | `"1.5s"` | How often the preview refreshes. |
| `event_log_lines` | `200` | Number of recent event log rows available in the UI. |
| `sidebar_width` | `32` | Configured sidebar width value. |

### `[retention]`

| Key | Default | Meaning |
| --- | --- | --- |
| `hint_threshold` | `6` | Show a cleanup hint when a project has more than this many finished sessions. |
| `prune_keep_default` | `5` | Default number of finished sessions to keep per project during `cleo prune`. |
| `idle_to_completed_timeout` | `"10m"` | Reconciler timeout that moves idle sessions toward completed. |
| `spawning_timeout` | `"30s"` | Timeout used to detect sessions that never finish startup. |

## Hooks And State Tracking

Cleo learns session state from hook events emitted by supported agents.

Claude Code events installed by `cleo init`:

```text
SessionStart
UserPromptSubmit
PreToolUse
PostToolUse
Notification
Stop
SessionEnd
SubagentStop
```

Codex events installed by `cleo init`:

```text
SessionStart
UserPromptSubmit
PreToolUse
PostToolUse
PermissionRequest
Stop
```

Cleo starts tmux sessions with `CLEO_SESSION_ID` in the environment. Hooks use that value to attribute events to the right Cleo session. If the hook environment does not preserve that variable, Cleo falls back to the hook payload working directory and chooses the most recently started active session for the matching project and agent.

### Session States

| State | Meaning |
| --- | --- |
| `spawning` | Cleo created state and is starting tmux/agent process. |
| `running` | The agent is active or has recently resumed work. |
| `waiting_for_input` | The agent requested input, approval, or attention. |
| `idle` | The agent stopped its current turn but the session is still available. |
| `completed` | The session ended cleanly or aged out from idle. |
| `error` | Cleo recorded an error state. |
| `dead` | The underlying tmux session is gone. |

The reconciler can synthesize some transitions, such as marking missing tmux sessions as `dead` or moving long-idle sessions toward `completed`.

## Files Cleo Manages

By default, Cleo stores runtime files under:

```text
~/.config/cleo/
```

If `XDG_CONFIG_HOME` is set, the root is:

```text
$XDG_CONFIG_HOME/cleo/
```

Important files:

| Path | Purpose |
| --- | --- |
| `config.toml` | User configuration. |
| `projects.json` | Registered projects. |
| `state.json` | Current known sessions. |
| `state.json.lock` | State file lock. |
| `focus.json` | Best-effort tmux session focus state for sound suppression. |
| `events/<session-id>.jsonl` | Per-session event log. |
| `events/archive/` | Archived event logs from pruned sessions. |
| `sounds/` | Sound assets used by `[sound.events]`. |
| `hook-errors.log` | Hook handler errors. |
| `hook-trace.log` | Hook attribution trace used by `cleo doctor`. |

Agent hook files live in the agent-specific config directories:

| Path | Purpose |
| --- | --- |
| `~/.claude/settings.json` | Claude Code hooks. |
| `~/.codex/hooks.json` | Codex hooks. |
| `~/.codex/config.toml` | Codex hooks feature flag. |

## Troubleshooting

### `cleo init` reports a hook conflict

The target agent config already has a different hook entry for the same event. Review the file manually, or rerun:

```bash
cleo init --force
```

`--force` overwrites conflicting hook entries for Cleo-managed events.

### Codex hooks are installed but nothing updates

Run:

```bash
cleo doctor
```

Then open Codex and run:

```text
/hooks
```

Approve the Cleo hook names if Codex lists them under review. Restart any Codex sessions that were already open before `cleo init`, because they may not have loaded the updated `~/.codex/config.toml`.

### Sessions stay `running`

For agents configured with `hooks = "none"`, this is expected. Cleo can manage the tmux session but cannot observe fine-grained lifecycle events.

For Claude Code or Codex, run `cleo doctor` and check `~/.config/cleo/hook-trace.log` and `~/.config/cleo/hook-errors.log`.

### A session is `dead`

The tmux session no longer exists. This can happen if tmux was killed, the session exited, or another tool removed it. Run:

```bash
cleo prune
```

to clean up finished state.

### No sound plays

Check:

```bash
cleo init
ls ~/.config/cleo/sounds
```

On macOS, also check:

```bash
which afplay
```

You can disable sound entirely:

```toml
[sound]
enabled = false
```

### Cleo binary moved after hooks were installed

Hooks store the absolute path to the Cleo executable. Re-run:

```bash
cleo init
```

so hook files point at the current binary path.

### Project was registered with the wrong path

Remove and re-add it:

```bash
cleo rm old-project-id
cleo add /correct/path
```

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
