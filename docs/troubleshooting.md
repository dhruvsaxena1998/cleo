# Troubleshooting

Common failure modes and how to recover. For installation problems, start with [Installation › Verify the install](installation.md#verify-the-install).

## `cleo hooks init` reports a hook conflict

The target agent config already has a different hook entry for the same event. Review the file manually, or rerun:

```bash
cleo hooks init --force
```

`--force` overwrites conflicting hook entries for Cleo-managed events.

## Codex hooks are installed but nothing updates

Run:

```bash
cleo doctor
```

Then open Codex and run:

```text
/hooks
```

Approve the Cleo hook names if Codex lists them under review. Restart any Codex sessions that were already open before `cleo hooks init`, because they may not have loaded the updated `~/.codex/config.toml`.

## Sessions stay `running`

For custom agents without a Cleo hook integration, this can be expected. Cleo can manage the tmux session but may not observe fine-grained lifecycle events.

For Claude Code, Codex, Pi, or OpenCode, run `cleo doctor` and check `~/.config/cleo/hook-trace.log` and `~/.config/cleo/hook-errors.log`.

## A session is `dead`

The tmux session no longer exists. This can happen if tmux was killed, the session exited, or another tool removed it. Run:

```bash
cleo prune
```

to clean up finished state.

## No sound plays

Check:

```bash
cleo hooks init
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

## Cleo binary moved after hooks were installed

Hooks store the absolute path to the Cleo executable. Re-run:

```bash
cleo hooks init
```

so hook files point at the current binary path.

## Project was registered with the wrong path

Remove and re-add it:

```bash
cleo rm old-project-id
cleo add /correct/path
```

## `tmux: command not found`

Cleo doesn't ship tmux; it talks to your installed copy. Install tmux 3.0+ via Homebrew:

```bash
brew install tmux
```

## Still stuck?

Open an issue at [github.com/dhruvsaxena1998/cleo/issues](https://github.com/dhruvsaxena1998/cleo/issues) with the output of `cleo --version`, `tmux -V`, `cleo doctor`, and your platform.
