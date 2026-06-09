# Configuration

Cleo reads and writes:

```text
~/.config/cleo/config.toml
```

If `XDG_CONFIG_HOME` is set, Cleo uses:

```text
$XDG_CONFIG_HOME/cleo/config.toml
```

The config file is created on first run with defaults. You can edit it by hand.

## Default Config Shape

```toml
default_agent = "claude"

[tmux]
detach_key = "C-b d"

[sound]
enabled = true
volume = 0.7

[sound.events.session_start]
file = "start.wav"
enabled = true

[sound.events.needs_input]
file = "attention.wav"
enabled = true

[sound.events.session_idle]
file = "done.wav"
enabled = true

[sound.events.session_completed]
file = "done.wav"
enabled = true

[sound.events.session_error]
file = "error.wav"
enabled = true

[agents.claude]
command = "claude"
label = "cl"
color = "#CC785C"

[agents.codex]
command = "codex"
label = "cx"
color = "#10A37F"

[agents.opencode]
command = "opencode"
label = "oc"
color = "#FF6B35"

[agents.pi]
command = "pi"
label = "pi"
color = "#7C3AED"

[ui]
theme = "catppuccin-mocha"
icons = "nerd"
editor = ""
sidebar_width = 48
event_log_lines = 200
status_timeout_seconds = 3.0

[ui.pane_preview]
enabled = true
lines = 30
interval = "2s"

[ui.mouse]
enabled = true

[timeouts]
idle_to_completed_timeout = "10m"
spawning_timeout = "30s"

[pruning]
hint_threshold = 6
keep_default = 5
```

Durations use Go duration strings such as `"500ms"`, `"1.5s"`, `"30s"`, `"10m"`, or `"1h"`.

## Top-level keys

| Key | Default | Meaning |
| --- | --- | --- |
| `default_agent` | `"claude"` | Default agent name for flows that need one. |

## `[tmux]`

| Key | Default | Meaning |
| --- | --- | --- |
| `detach_key` | `"C-b d"` | Tmux detach key Cleo tries to bind for spawned sessions. |

## `[sound]`

| Key | Default | Meaning |
| --- | --- | --- |
| `enabled` | `true` | Enables sound playback for hook-triggered state transitions. |
| `volume` | `0.7` | Playback volume in `0.0`–`1.0`. Applied on macOS via `afplay -v`; Linux players (`paplay`/`aplay`/`play`) ignore this value, so use system mixer there. |

### `[sound.events.<event>]`

Each sound event has a `file` and `enabled` field. Relative paths are resolved under:

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

`cleo hooks init` extracts bundled default WAV files into the sounds directory. Set an event's `enabled` field to `false` to mute that event while keeping its file mapping configured.

| Event | Default |
| --- | --- |
| `session_start` | `true` |
| `needs_input` | `true` |
| `session_idle` | `true` |
| `session_completed` | `true` |
| `session_error` | `true` |

For example, to keep attention and error sounds but disable startup and completion sounds:

```toml
[sound.events.session_start]
enabled = false

[sound.events.session_completed]
enabled = false
```

Partial sound event configs are supported: if you override only `enabled`, Cleo keeps the default file. You can also disable all sounds at once with `sound.enabled = false`.

### Focus-aware sound suppression

Cleo suppresses session sounds while that exact tmux session is focused. This prevents duplicate attention sounds when you are already attached to the agent and watching it work.

When Cleo starts or attaches to a tmux session, it enables tmux `focus-events` and installs tmux client focus hooks. Those hooks update Cleo's local focus state on attach, detach, focus-in, and focus-out. If you switch from the terminal to another app such as VS Code or Chrome, tmux can emit focus-out and Cleo will resume playing notification sounds for that session.

This is best-effort because terminal focus reporting depends on tmux and terminal emulator support. If Cleo cannot determine focus, it plays sounds rather than risk hiding an alert.

## `[agents.<name>]`

Defines agents you can pass to `cleo run <agent>`.

| Key | Meaning |
| --- | --- |
| `command` | Executable command Cleo starts inside tmux. |
| `label` | Short label shown in compact UI surfaces. |
| `color` | Hex color used by the TUI for that agent. |

Example custom agent:

```toml
[agents.myagent]
command = "myagent"
label = "ma"
color = "#3B82F6"
```

After this, run:

```bash
cleo run myagent --name investigate-cache
```

Hook support is installed by `cleo hooks init` for supported agents. Custom agents can still be spawned and managed through tmux; without a hook integration, Cleo relies on tmux reconciliation rather than fine-grained lifecycle events.

## `[ui]`

| Key | Default | Meaning |
| --- | --- | --- |
| `editor` | `""` | Optional editor command for Dashboard `ctrl+g`; falls back to `$EDITOR` when empty. |
| `event_log_lines` | `200` | Number of recent event log rows available in the UI. |
| `icons` | `"nerd"` | Glyph set used throughout the TUI. See list below. |
| `sidebar_width` | `48` | Sidebar width in character columns (10–200). |
| `status_timeout_seconds` | `3.0` | Seconds a Dashboard status bar message stays visible before it auto-expires. Fractional values allowed; clamped to 0.5–10. |
| `theme` | `"catppuccin-mocha"` | Color theme used by the TUI. See list below. |

### `[ui.pane_preview]`

| Key | Default | Meaning |
| --- | --- | --- |
| `enabled` | `true` | Show tmux pane output previews in the dashboard. |
| `lines` | `30` | Number of tmux pane lines to capture for preview. |
| `interval` | `"2s"` | How often the preview refreshes. |

### `[ui.mouse]`

| Key | Default | Meaning |
| --- | --- | --- |
| `enabled` | `true` | Gated terminal mouse-tracking mode (scrolling, clicking). When disabled, native click-and-drag text selection and copying keep working. |

### Themes

`theme` accepts one of:

| Name | Style | Preview |
| --- | --- | --- |
| `catppuccin-mocha` | Default. Warm, low-contrast pastel dark theme. | ![catppuccin-mocha](../screenshots/theme-catppuccin-mocha.png) |
| `gruvbox-dark` | Retro warm dark theme. | ![gruvbox-dark](../screenshots/theme-gruvbox-dark.png) |
| `onedark` | Atom-style cool dark theme. | ![onedark](../screenshots/theme-onedark.png) |
| `void` | High-contrast minimal monochrome. | ![void](../screenshots/theme-void.png) |
| `synthwave` | Vivid magenta/cyan neon dark theme. | ![synthwave](../screenshots/theme-synthwave.png) |

Unknown values fall back to `catppuccin-mocha` and are reported as config warnings. To preview a theme without committing to it, open the in-app settings editor (`,`) and cycle the **theme** field with `←`/`→` — the dashboard recolors live; press `esc` to discard or `enter` to save (see [Editing the config](#editing-the-config)).

### Icons

`icons` selects the glyph set used for session-state markers, the project tree, and the topbar/footer/panel chrome:

| Name | Style |
| --- | --- |
| `nerd` | Default. [Nerd Font](https://www.nerdfonts.com/) glyphs (folder, git-branch, status, clock…). Requires a patched font in your terminal, or these render as missing-glyph boxes. |
| `unicode` | Broadly-portable symbols (`●◑○✓✗`, `▸▾`). Use this if you do not have a Nerd Font installed. |
| `ascii` | Plain ASCII fallback (`* ? o + x`, `> v`) for terminals that mangle wider Unicode. |

Unknown values fall back to `nerd`. The setting takes effect on the next launch.

## `[timeouts]`

| Key | Default | Meaning |
| --- | --- | --- |
| `idle_to_completed_timeout` | `"10m"` | Reconciler timeout that moves idle sessions toward completed. |
| `spawning_timeout` | `"30s"` | Timeout used to detect sessions that never finish startup. |

## `[pruning]`

| Key | Default | Meaning |
| --- | --- | --- |
| `hint_threshold` | `6` | Show a cleanup hint when a project has more than this many finished sessions. |
| `keep_default` | `5` | Default number of finished sessions to keep per project during `cleo prune`. |

## `[keybinds]`

Rebind main-view actions. Each entry maps an action name to a list of keys:

```toml
[keybinds]
  up = ["w"]            # move up with 'w' instead of the defaults
  down = ["s"]
  kill = ["x", "ctrl+k"]
```

Resolution rules:

- **Per-action replace** — listing an action replaces its keys entirely (the defaults for that action no longer apply).
- **Omitted action** — keeps its default keys.
- **Empty list** (`up = []`) — reverts that action to its default. There is no disabled state; every action always resolves to at least one key.
- **Validation** — each key is checked against a best-effort allowlist (special keys like `up`/`down`/`enter`/`tab`, the `ctrl+`/`alt+` prefixes, and single printable characters). An unrecognized key (e.g. `ctrl-k`, `Enter`, `cmd+k`) is dropped from that action's list with a warning; the other valid keys in the same list survive. If every key you configured for an action is dropped, that action quietly falls back to its built-in default.
- **Conflict precedence** — when two actions claim the same key, the action **higher in the table below keeps it** (first-wins by importance); the key is dropped from the later action, which keeps its remaining keys. The table order _is_ the precedence ranking.
- **Reserved keys** — `enter`, `esc`, and `ctrl+c` always perform attach/confirm, cancel, and quit in every mode and cannot be reassigned to another action (a config such as `kill = ["enter"]` is warned and ignored). The hatches work regardless of config, so you can never lock yourself out.

Any of these adjustments are surfaced in a popup the first time you launch `cleo` after editing the config, and are also listed by `cleo doctor`.

Within-popup keys (text-input editing, spawn-field tab cycling, finder query typing) are not configurable.

Action names and their defaults:

| Action | Default | Meaning |
| --- | --- | --- |
| `up` | `["up", "k"]` | Move cursor up. |
| `down` | `["down", "j"]` | Move cursor down. |
| `attach` | `["enter"]` | Attach to the selected session (reserved). |
| `expand` | `[" "]` | Expand or collapse the focused project. |
| `find` | `["/"]` | Open the finder. |
| `new` | `["n"]` | Start a new session. |
| `view` | `["v"]` | View a session without attaching. |
| `send` | `["m"]` | Send text to the selected session. |
| `editor` | `["ctrl+g", "e"]` | Open the selected project in your editor. |
| `rename` | `["r"]` | Rename a session. |
| `kill` | `["K", "ctrl+k"]` | Kill the selected session. |
| `prune` | `["P"]` | Prune finished sessions for the focused project. |
| `remove` | `["D"]` | Remove the focused project. |
| `mute` | `["alt+m"]` | Toggle sound for the running Cleo process. |
| `settings` | `[","]` | Open the in-app settings editor. |
| `help` | `["?"]` | Show the help popup. |
| `quit` | `["q"]` | Quit the dashboard. |
| `close` | `["esc"]` | Cancel the current popup/filter (reserved). |

## Editing the config

### In-app settings editor

Press `,` in the dashboard to open the settings editor. It covers the settings you tweak most often without leaving the TUI:

- **General** — `default_agent`, `editor`
- **Appearance** — `theme`, `sidebar_width`
- **Pane Preview** — `enabled`, `lines`, `interval`
- **UX** — `status_timeout_seconds`, `event_log_lines`
- **Timeouts** — `idle_to_completed_timeout`, `spawning_timeout`
- **Pruning** — `hint_threshold`, `keep_default`
- **Sound** — `enabled`, `volume`, plus a per-event on/off toggle for each `[sound.events.<event>]`

Navigate fields with `↑`/`↓` (or `Tab`), change the focused field with `←`/`→` (toggle a switch, cycle an option, step a number); on the `editor` field just type. The list scrolls when it doesn't fit the terminal, keeping the cursor in view. Changes preview live — the theme recolors and the sidebar resizes as you scroll. Press `enter` to write them to `config.toml`, or `esc` to discard everything and revert. Out-of-range values are clamped on save, exactly as they would be on load.

The `[tmux]`, `[agents]`, `[ui.mouse]`, and `[keybinds]` tables are still edited in the file. Per-event sound **files** (`[sound.events.<event>].file`) are also file-only — the panel toggles each event on or off but does not change which sound it plays.

### Editing the file

Cleo reads the config on startup. After editing `config.toml`, quit the dashboard (`q`) and relaunch `cleo` to pick up changes. Hook handlers also re-read the config on each invocation, so sound and event toggles take effect on the next hook event without restarting anything.

If you delete or corrupt the file, the next `cleo` run rewrites it from defaults — your projects and session state are kept in separate files (`projects.json`, `state.json`) and are not affected.

Partial configs are supported: any unspecified key is filled in from defaults at load time. You can keep your `config.toml` minimal and only override what you actually want to change.

## Configuration recipes

The defaults are reasonable, so most users only need a few overrides. Here are common patterns.

### Silent mode

Disable all sound playback without removing the event mappings:

```toml
[sound]
enabled = false
```

### Only attention-worthy sounds

Mute routine start/idle/completion sounds, keep the ones that signal you should look at the terminal:

```toml
[sound.events.session_start]
enabled = false

[sound.events.session_idle]
enabled = false

[sound.events.session_completed]
enabled = false
```

### Lower default volume (macOS)

```toml
[sound]
enabled = true
volume = 0.3
```

On Linux, set the system mixer instead — the `volume` value is ignored.

### Custom sound files

Drop your own `.wav` files into `~/.config/cleo/sounds/` and reference them by basename, or use absolute paths:

```toml
[sound.events.needs_input]
file = "my-attention.wav"

[sound.events.session_error]
file = "/Users/me/sounds/error-loud.wav"
```

### Switch theme

```toml
[ui]
theme = "synthwave"
```

### Add a new agent

Cleo can manage any tmux-runnable command. Without a hook protocol, Cleo can spawn / attach / kill / prune the session but cannot observe fine-grained lifecycle (it stays `running` until tmux exits). Example for [`aider`](https://aider.chat):

```toml
[agents.aider]
command = "aider"
label = "ai"
color = "#9333EA"
```

Then:

```bash
cleo run aider --name refactor-payments
```

### Override an existing agent's command

To run Claude Code with extra flags, override the `command`:

```toml
[agents.claude]
command = "claude --dangerously-skip-permissions"
label = "cl"
color = "#CC785C"
```

The whole string is passed to tmux's `new-session`, so flags and arguments are preserved.

### Open Projects from the Dashboard in your editor

Press `ctrl+g` on a Project row, or on any Session row, to open that Project directory. `e` also works as a secondary binding. Cleo uses `[ui].editor` first, then `$EDITOR`. GUI launchers such as `code`, `cursor`, `zed`, `subl`, `open`, `mate`, `bbedit`, and common JetBrains launchers open detached. Terminal editors such as `nvim`, `vim`, `emacs`, `hx`, `micro`, and `nano` take over the terminal until they exit.

```toml
[ui]
editor = "code --reuse-window"
```

### Larger preview pane / more UI breathing room

```toml
[ui]
sidebar_width = 40
event_log_lines = 500

[ui.pane_preview]
lines = 60
interval = "0.75s"
```

Increasing `ui.pane_preview.interval` reduces tmux pane capture traffic; decreasing it makes the preview feel snappier at the cost of a few more `capture-pane` calls per second.

### Status messages that linger longer (or clear faster)

Dashboard status bar messages — confirmations, errors, and blocked-action hints — auto-expire after `ui.status_timeout_seconds`. Increase it if you read slowly or use accessibility tooling; decrease it to bring the normal footer hints back sooner:

```toml
[ui]
status_timeout_seconds = 6      # keep messages up longer
# status_timeout_seconds = 0.5  # minimum; snappier footer
```

Values are in seconds (fractional allowed) and clamped to the 0.5–10 range; out-of-range values are clamped with a config warning. Pressing `esc` always clears a status message immediately, regardless of the timeout.

### Tighter pruning and timeouts

Auto-promote idle sessions to `completed` faster, and keep fewer finished sessions per project after `cleo prune`:

```toml
[timeouts]
idle_to_completed_timeout = "2m"
spawning_timeout = "15s"

[pruning]
hint_threshold = 3
keep_default = 2
```

### Change the default agent

Used by flows that need an agent but don't get one explicitly:

```toml
default_agent = "codex"
```

## Environment Variables

Cleo reads a small number of environment variables:

| Variable | Effect |
| --- | --- |
| `XDG_CONFIG_HOME` | Overrides the config root. When set, Cleo uses `$XDG_CONFIG_HOME/cleo/` instead of `~/.config/cleo/`. |
| `CLEO_SESSION_ID` | Set automatically by `cleo run` inside the spawned tmux session. Hook handlers read this to attribute events to the correct Cleo session. You normally do not set this yourself. |
| `TMUX` | Standard tmux variable. Cleo checks it to detect that you are already inside tmux when running `cleo run`, so it can `switch-client` rather than nest a new server. |
