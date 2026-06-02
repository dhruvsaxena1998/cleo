# Aliases

`cleo` (no args) opens the TUI, where you navigate to a project, then spawn a session. `cleo run <agent>` spawns directly from the command line without the TUI, but still takes several keystrokes.

**The pattern**: map a short alias straight to `cleo run <agent> --yes`. CWD is detected automatically - no path needed. The alias lands you directly in a running agent session with one word.

## Shell aliases

Add one of these to your `.zshrc`, `.bashrc`, or equivalent:

```sh
alias cc='cleo run claude --yes'
alias cp='cleo run pi --yes'
alias co='cleo run codex --yes'
```

Then from any project directory:

```
~/my-project
$ cc
spawned cleo-my-project-claude-witty-koala
# tmux attaches immediately
```

## How it works

- `cleo run <agent>` resolves the current working directory to a Project
- `--yes` skips the "[Y/n]" prompt on first visit, auto-registering the directory
- After registration, `--yes` is a no-op (the Project already exists)
- A Session spawns in tmux and you attach to it

If no agent matches or tmux isn't available, `cleo run` surfaces the error as usual.

## Caveat: project list noise

Every directory you run the alias from gets permanently registered in `projects.json`. Visiting `/tmp/scratch` once adds it to your project list forever.

Clean up with:

```sh
cleo ls           # see project IDs
cleo rm <id>      # remove a project and its sessions
```

## Advanced: default agent per project

If you want a single alias that picks the right agent for each project, use a shell function that reads the project's `default_agent` from `projects.json`:

```sh
# Requires jq. Falls back to claude if no default_agent is set.
cc() {
  local dir
  dir=$(pwd)
  local agent
  agent=$(jq -r --arg d "$dir" '.projects[] | select(.path == $d) | .default_agent // "claude"' ~/.cleo/projects.json 2>/dev/null)
  agent="${agent:-claude}"
  cleo run "$agent"
}
```

Set `default_agent` per project with:

```sh
# Edit ~/.cleo/projects.json and add "default_agent": "pi" to a project entry
```
