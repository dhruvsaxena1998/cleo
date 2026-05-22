# Cleo config manual test cases

Use this checklist after changing `~/.config/cleo/config.toml`.

## Automated Coverage

Most deterministic config behavior is covered by Go tests:

```sh
go test ./internal/config ./internal/cli ./internal/hooks ./internal/tui
```

Covered:

- all config keys decode from TOML and valid values do not warn
- invalid scalar values clamp and warn
- partial `sound.events` and `agents` overrides preserve defaults
- startup extracts bundled sound assets
- `cleo run` uses the configured agent command
- `cleo prune` uses `pruning.keep_default`
- hook sound dispatch respects event `enabled` and `file`
- pane preview respects `enabled` and `lines`
- config warnings populate the TUI startup status
- `cleo doctor` prints config warnings

Still manual:

- audible volume differences
- visual theme/sidebar/color rendering quality
- tmux detach key behavior
- real external hook installation drift after `cleo init`

## Setup

Build and install the current binary:

```sh
go install ./cmd/cleo
```

For a clean-app test:

```sh
rm -rf ~/.config/cleo
cleo
```

Expected:

- `~/.config/cleo/config.toml` exists.
- `~/.config/cleo/sounds/start.wav` exists.
- `~/.config/cleo/sounds/attention.wav` exists.
- `~/.config/cleo/sounds/done.wav` exists.
- `~/.config/cleo/sounds/error.wav` exists.

After editing config, restart `cleo` or run a new hook process. Config is loaded at process startup.

## Config Shape

### Default file shape

Edit: none.

Action:

```sh
rm -rf ~/.config/cleo
cleo
```

Expected:

- `config.toml` contains `default_agent = "claude"`.
- It contains `[tmux]`, `[sound]`, `[agents.claude]`, `[ui]`, `[ui.pane_preview]`, `[timeouts]`, and `[pruning]`.
- It does not contain `[defaults]`, `[retention]`, `event_enabled`, or agent `hooks`.

## Top-Level Settings

### `default_agent`

Edit:

```toml
default_agent = "codex"
```

Action:

```sh
cleo
```

Expected:

- No warning is printed.
- Current implementation has no runtime callsite for this setting. Treat any visible default-agent behavior as not yet implemented.

## `[tmux]`

### `tmux.detach_key`

Edit:

```toml
[tmux]
  detach_key = "C-b x"
```

Action:

```sh
cleo run claude
```

Expected:

- The tmux detach binding uses the last key in the setting, `x`.
- Attaching to the spawned session and pressing the configured prefix/key detaches.

## `[sound]`

### `sound.enabled`

Edit:

```toml
[sound]
  enabled = false
```

Action:

- Trigger a hook that normally plays sound, such as Claude `SessionStart` or `Stop`.
- Check trace:

```sh
tail -n 20 ~/.config/cleo/hook-trace.log
```

Expected:

- No sound plays.
- Trace row for the sound event has `"reason":"disabled"`.

Then set:

```toml
[sound]
  enabled = true
```

Expected:

- Same hook event logs `"reason":"played"` unless the session is focused or idle-nudge suppressed.

### `sound.volume`

Edit:

```toml
[sound]
  enabled = true
  volume = 0.1
```

Action:

- Trigger a sound event.
- Then change to `volume = 1.0` and trigger the same event.

Expected:

- On macOS, the `1.0` event is louder than `0.1`.
- No config warning appears for values between `0` and `1`.

### `sound.volume` validation

Edit:

```toml
[sound]
  volume = 2.5
```

Action:

```sh
cleo doctor --quiet
```

Expected:

- Startup stderr includes `sound.volume above 1; clamped to 1`.
- `cleo doctor` lists the same config warning.

## `[sound.events.<event>]`

Run each event with default config first:

| Event | Trigger | Expected file |
| --- | --- | --- |
| `session_start` | start a tracked agent session | `start.wav` |
| `needs_input` | trigger a real notification / permission request while session is not focused | `attention.wav` |
| `session_idle` | agent emits stop/idle | `done.wav` |
| `session_completed` | agent session ends | `done.wav` |
| `session_error` | OpenCode error event | `error.wav` |

For each event, check:

```sh
tail -n 20 ~/.config/cleo/hook-trace.log
```

Expected:

- Matching `"sound_event":"<event>"`.
- `"reason":"played"` when not focused, disabled, or idle-nudge suppressed.

### Event `enabled`

Edit one event:

```toml
[sound.events.session_idle]
  enabled = false
```

Action:

- Trigger session idle.

Expected:

- No sound plays.
- Trace row has `"sound_event":"session_idle"` and `"reason":"disabled"`.
- Other sound events still play.

### Event `file`

Edit one event:

```toml
[sound.events.session_start]
  file = "error.wav"
  enabled = true
```

Action:

- Start a tracked agent session.

Expected:

- The error sound plays for session start.
- Trace still records `"sound_event":"session_start"` and `"reason":"played"`.

### Partial event override

Edit only:

```toml
[sound.events.session_completed]
  enabled = false
```

Action:

- Trigger session completed.
- Trigger session start.

Expected:

- `session_completed` is disabled.
- `session_completed.file` still defaults to `done.wav`.
- `session_start` still plays `start.wav`.

## `[agents.<name>]`

### Agent `command`

Edit:

```toml
[agents.claude]
  command = "echo hello-from-cleo"
  label = "cl"
  color = "#CC785C"
```

Action:

```sh
cleo run claude --no-attach --yes
tmux capture-pane -pt "$(tmux list-sessions -F '#S' | grep '^cleo-' | tail -1)"
```

Expected:

- The tmux pane output includes `hello-from-cleo`.

### Agent `label`

Edit:

```toml
[agents.claude]
  label = "zz"
```

Action:

```sh
cleo
```

Expected:

- Claude sessions show the `zz` badge in the sidebar/main pane.
- Command and color are preserved from defaults if omitted.

### Agent `color`

Edit:

```toml
[agents.claude]
  color = "#ffffff"
```

Action:

```sh
cleo
```

Expected:

- Claude badge color changes to white.
- Command and label are preserved from defaults if omitted.

### Empty agent command fallback

Edit:

```toml
[agents.test]
  command = ""
  label = "tt"
  color = "#ffffff"
```

Action:

```sh
cleo run test --no-attach --yes
```

Expected:

- Cleo attempts to run command `test`.
- No config warning is emitted for empty command; it falls back to the agent key.

## `[ui]`

### `ui.theme`

Edit one valid theme:

```toml
[ui]
  theme = "gruvbox-dark"
```

Action:

```sh
cleo
```

Expected:

- TUI colors change to the selected theme.
- Valid themes: `catppuccin-mocha`, `gruvbox-dark`, `onedark`, `void`, `synthwave`.

### `ui.theme` validation

Edit:

```toml
[ui]
  theme = "bad-theme"
```

Action:

```sh
cleo doctor --quiet
```

Expected:

- Warning says the theme is unknown and falls back to `catppuccin-mocha`.

### `ui.sidebar_width`

Edit:

```toml
[ui]
  sidebar_width = 10
```

Action:

```sh
cleo
```

Expected:

- Sidebar is narrow.

Then edit:

```toml
[ui]
  sidebar_width = 60
```

Expected:

- Sidebar is visibly wider.

### `ui.sidebar_width` validation

Edit:

```toml
[ui]
  sidebar_width = 2
```

Expected:

- Warning says it was clamped to `10`.

Edit:

```toml
[ui]
  sidebar_width = 300
```

Expected:

- Warning says it was clamped to `200`.

### `ui.event_log_lines`

Edit:

```toml
[ui]
  event_log_lines = 10
```

Action:

```sh
cleo
```

Expected:

- Main pane event log shows fewer historical event rows than default.

### `ui.event_log_lines` validation

Edit:

```toml
[ui]
  event_log_lines = 1
```

Expected:

- Warning says it was clamped to `10`.

## `[ui.pane_preview]`

### `ui.pane_preview.enabled`

Edit:

```toml
[ui.pane_preview]
  enabled = false
```

Action:

```sh
cleo
```

Expected:

- Moving between running sessions does not auto-refresh pane preview content.

Set it back to `true`.

Expected:

- Pane preview auto-refreshes again.

### `ui.pane_preview.lines`

Edit:

```toml
[ui.pane_preview]
  lines = 5
```

Action:

```sh
cleo
```

Expected:

- Preview captures roughly the last 5 lines of the selected tmux pane.

### `ui.pane_preview.lines` validation

Edit:

```toml
[ui.pane_preview]
  lines = 0
```

Expected:

- Warning says it was clamped to `1`.

### `ui.pane_preview.interval`

Edit:

```toml
[ui.pane_preview]
  interval = "100ms"
```

Action:

```sh
cleo
```

Expected:

- Pane preview refreshes quickly.

Then edit:

```toml
[ui.pane_preview]
  interval = "5s"
```

Expected:

- Pane preview refreshes slowly.

### `ui.pane_preview.interval` validation

Edit:

```toml
[ui.pane_preview]
  interval = "5ms"
```

Expected:

- Warning says it was clamped to `100ms`.

## `[timeouts]`

### `timeouts.idle_to_completed_timeout`

Edit:

```toml
[timeouts]
  idle_to_completed_timeout = "2s"
```

Action:

- Let an agent enter idle state.
- Wait at least 2 seconds.
- Run:

```sh
cleo ls
```

Expected:

- Idle session transitions to completed.

### `timeouts.idle_to_completed_timeout` validation

Edit:

```toml
[timeouts]
  idle_to_completed_timeout = "1ms"
```

Expected:

- Warning says it was clamped to `100ms`.

### `timeouts.spawning_timeout`

Edit an agent command that stays in spawning or cannot complete startup, then:

```toml
[timeouts]
  spawning_timeout = "2s"
```

Action:

```sh
cleo ls
```

Expected:

- A session stuck in spawning is reconciled according to the state-machine timeout behavior after roughly 2 seconds.

### `timeouts.spawning_timeout` validation

Edit:

```toml
[timeouts]
  spawning_timeout = "1ms"
```

Expected:

- Warning says it was clamped to `100ms`.

## `[pruning]`

### `pruning.hint_threshold`

Edit:

```toml
[pruning]
  hint_threshold = 1
```

Action:

- Create at least 2 finished sessions in one project.
- Open `cleo`.

Expected:

- TUI shows the pruning hint banner for that project.

### `pruning.keep_default`

Edit:

```toml
[pruning]
  keep_default = 1
```

Action:

- Create at least 3 finished sessions in one project.
- Run:

```sh
cleo prune <project> --dry-run --yes
```

Expected:

- Dry run lists all but the most recent finished session for that project.

## Warning Surface

Edit multiple invalid values:

```toml
[sound]
  volume = 9

[ui]
  theme = "bad-theme"
  sidebar_width = 2
```

Action:

```sh
cleo doctor --quiet
```

Expected:

- Startup stderr prints each config warning.
- TUI startup status shows `config warnings: run cleo doctor`.
- `cleo doctor` lists the full warnings under `Config warnings:`.
