# Hooks & State Tracking

Cleo learns session state from hook events emitted by supported agents. Install them with `cleo hooks init` (see [Commands](commands.md#cleo-hooks-init)).

Claude Code events installed by `cleo hooks init`:

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

Codex events installed by `cleo hooks init`:

```text
SessionStart
UserPromptSubmit
PreToolUse
PostToolUse
PermissionRequest
Stop
```

Pi events installed by `cleo hooks init`:

```text
session_start
input
tool_call
tool_result
agent_end
session_shutdown
```

OpenCode events installed by `cleo hooks init`:

```text
session.created
tool.execute.before
tool.execute.after
permission.asked
session.idle
session.deleted
session.error
```

Cleo starts tmux sessions with `CLEO_SESSION_ID` in the environment. Hooks use that value to attribute events to the right Cleo session. If the hook environment does not preserve that variable, Cleo falls back to the hook payload working directory and chooses the most recently started active session for the matching project and agent.

## Session States

| State | Meaning |
| --- | --- |
| `spawning` | Cleo created state and is starting tmux/agent process. |
| `running` | The agent is active or has recently resumed work. |
| `waiting_for_input` | The agent requested input, approval, or attention. |
| `idle` | The agent stopped its current turn but the session is still available. |
| `completed` | The session ended cleanly or aged out from idle. |
| `error` | Cleo recorded an error state. |
| `dead` | The underlying tmux session is gone. |

The reconciler can synthesize some transitions, such as marking missing tmux sessions as `dead` or moving long-idle sessions toward `completed`. Timeouts for these transitions are configurable under [`[timeouts]`](configuration.md#timeouts).

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
| `~/.pi/agent/extensions/cleo.ts` | Pi extension. |
| `~/.config/opencode/plugins/cleo.ts` | OpenCode plugin. |
