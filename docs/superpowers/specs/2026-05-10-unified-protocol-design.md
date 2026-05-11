# Unified Protocol Interface — Design Spec

**Date:** 2026-05-10
**Status:** Draft (pending user review)
**Theme:** *Add Pi support; make adding any future agent a one-file exercise.*

---

## Motivation

Cleo currently has two hardcoded protocol branches in `handler.go` (`handleClaude`, `handleCodex`) and two parallel install paths in `install.go`. Adding Pi as a third agent would make it three parallel copies of the same pattern. This spec introduces a `Protocol` interface and a `NormalizedEvent` type so that `Handle()` never switches on protocol name again — each agent encapsulates its own parsing and the handler consumes a uniform event.

The scope is:

1. `Protocol` interface + `NormalizedEvent` type (new `protocol.go`)
2. Refactor Claude and Codex into the interface (no behaviour change)
3. Pi protocol — Go struct + TypeScript extension + `--payload` flag
4. `cleo init` output formatting pass

---

## 1. Core Types (`internal/hooks/protocol.go`)

### `NormalizedEvent`

The canonical representation every protocol produces after parsing its raw payload. The handler consumes only this type — it never reads protocol-specific fields.

```go
type NormalizedEvent struct {
    StateEvent state.Event // empty string = no state transition
    SoundEvent string      // empty = no sound
    Message    string      // Notification / PermissionRequest text
    ToolName   string      // written to the event log
    LogOnly    bool        // log entry only, no state transition (SubagentStop)
    LogType    string      // event log Type override when LogOnly=true; defaults to string(StateEvent)
}
```

### `Protocol` interface

```go
type Protocol interface {
    // Name returns the identifier used in "cleo hook <protocol> <event>".
    Name() string
    // Events returns the hook event names this protocol subscribes to.
    Events() []string
    // Install writes hooks into the agent's config file(s).
    Install(cleoBin string, force bool) error
    // Cleanup removes cleo-owned entries from the agent's config file(s).
    Cleanup() error
    // Normalize converts a raw event name and JSON payload into a NormalizedEvent.
    // Returns ok=false if the event is unknown and should be silently ignored.
    Normalize(event string, payload []byte) (NormalizedEvent, bool)
    // UsesCwdFallback returns true when the protocol may not propagate
    // CLEO_SESSION_ID to hook subprocesses. resolveSession() only calls
    // FindByCwd when this is true.
    UsesCwdFallback() bool
}
```

### `Protocols()`

The single place to register agents. Adding a new agent = one line here, plus its own file.

```go
func Protocols() []Protocol {
    return []Protocol{ClaudeProtocol{}, CodexProtocol{}, PiProtocol{}}
}
```

---

## 2. Refactored `Handle()` (`internal/hooks/handler.go`)

`Handle()` becomes a thin orchestrator. The two internal responsibilities — session resolution and event application — are extracted into named helpers.

```go
func Handle(d Deps, protocol, event string, payload []byte) error {
    proto, ok := findProtocol(Protocols(), protocol)
    if !ok {
        return fmt.Errorf("unknown protocol %q", protocol)
    }

    sid := resolveSession(d, proto, payload)
    if sid == "" {
        return nil // no session; already traced
    }

    norm, ok := proto.Normalize(event, payload)
    if !ok {
        return nil // unknown event for this protocol; silently ignore
    }

    return applyNormalized(d, sid, norm)
}
```

**`resolveSession(d Deps, proto Protocol, payload []byte) string`**

Extracted from the current inline logic. Behaviour is identical:
1. Read `CLEO_SESSION_ID` from env.
2. Validate the ID exists in `state.json`; on miss set `fallback_reason = env_unknown_session`.
3. If env var absent or invalid **and** `proto.UsesCwdFallback()` is true, call `d.FindByCwd(cwd, proto.Name())`.
4. Log a `hookTrace` entry regardless of outcome.
5. Return `""` on no match (caller returns nil).

**`applyNormalized(d Deps, sid string, norm NormalizedEvent) error`**

```go
func applyNormalized(d Deps, sid string, norm NormalizedEvent) error {
    if !norm.LogOnly {
        if _, err := d.State.Apply(sid, norm.StateEvent, norm.Message); err != nil {
            return err
        }
    }
    entryType := string(norm.StateEvent)
    if norm.LogType != "" {
        entryType = norm.LogType
    }
    _ = d.Events(sid).Append(events.Entry{
        Type:   entryType,
        Tool:   norm.ToolName,
        Detail: norm.Message,
    })
    if norm.SoundEvent != "" && d.Config.SoundEventEnabled(norm.SoundEvent) && !sessionFocused(d, sid) {
        playSound(d, norm.SoundEvent)
    }
    return nil
}
```

The old `handleClaude` and `handleCodex` functions are deleted.

---

## 3. Claude Protocol (`internal/hooks/claude.go`)

Pure extraction from `handler.go`. No behaviour change.

```go
type ClaudeProtocol struct{}

func (ClaudeProtocol) Name() string          { return "claude" }
func (ClaudeProtocol) Events() []string      { return claudeEvents }
func (ClaudeProtocol) UsesCwdFallback() bool { return false }

func (ClaudeProtocol) Normalize(event string, payload []byte) (NormalizedEvent, bool) {
    var p struct {
        ToolName string `json:"tool_name"`
        Message  string `json:"message"`
    }
    _ = json.Unmarshal(payload, &p)

    switch event {
    case "SessionStart":
        return NormalizedEvent{StateEvent: state.EvSessionStart, SoundEvent: "session_start", ToolName: p.ToolName}, true
    case "UserPromptSubmit":
        return NormalizedEvent{StateEvent: state.EvUserResume, ToolName: p.ToolName}, true
    case "PreToolUse":
        return NormalizedEvent{StateEvent: state.EvPreToolUse, ToolName: p.ToolName}, true
    case "PostToolUse":
        return NormalizedEvent{StateEvent: state.EvPostToolUse, ToolName: p.ToolName}, true
    case "Notification":
        return NormalizedEvent{StateEvent: state.EvNotification, SoundEvent: "needs_input", Message: p.Message, ToolName: p.ToolName}, true
    case "Stop":
        return NormalizedEvent{StateEvent: state.EvStop, SoundEvent: "session_idle", ToolName: p.ToolName}, true
    case "SessionEnd":
        return NormalizedEvent{StateEvent: state.EvSessionEnd, SoundEvent: "session_completed", ToolName: p.ToolName}, true
    case "SubagentStop":
        return NormalizedEvent{LogOnly: true, LogType: "SubagentStop", ToolName: p.ToolName}, true
    }
    return NormalizedEvent{}, false
}
```

`Install` and `Cleanup` delegate to the existing `InstallClaude` / `CleanupClaude` helpers in `install.go` (those helpers stay; only the call site moves).

---

## 4. Codex Protocol (`internal/hooks/codex.go`)

Thin wrapper: only `PermissionRequest` diverges from Claude's mapping.

```go
type CodexProtocol struct{}

func (CodexProtocol) Name() string          { return "codex" }
func (CodexProtocol) Events() []string      { return codexEvents }
func (CodexProtocol) UsesCwdFallback() bool { return true }

func (CodexProtocol) Normalize(event string, payload []byte) (NormalizedEvent, bool) {
    if event == "PermissionRequest" {
        return ClaudeProtocol{}.Normalize("Notification", synthesizeNotification(payload))
    }
    return ClaudeProtocol{}.Normalize(event, payload)
}
```

`synthesizeNotification(payload []byte) []byte` is a private helper (extracted from the current inline `handleCodex` logic) that builds a `claudePayload`-shaped JSON from the `PermissionRequest` fields (`tool_name`, `tool_input.command`, `tool_input.description`). Moving it to a named function makes it independently testable.

---

## 5. Pi Protocol (`internal/hooks/pi.go`)

### 5.1 Event mapping

Pi uses snake_case event names. There is no native "needs user attention" event (Pi has no Notification equivalent); `agent_end` is the turn-complete signal mapping to `EvStop → Idle`.

| Pi event | `NormalizedEvent` | Sound |
|---|---|---|
| `session_start` | `EvSessionStart` | `session_start` |
| `input` | `EvUserResume` | — |
| `tool_call` | `EvPreToolUse` | — |
| `tool_result` | `EvPostToolUse` | — |
| `agent_end` | `EvStop` | `session_idle` |
| `session_shutdown` | `EvSessionEnd` | `session_completed` |

```go
var piEvents = []string{
    "session_start", "input", "tool_call", "tool_result", "agent_end", "session_shutdown",
}

type PiProtocol struct{}

func (PiProtocol) Name() string          { return "pi" }
func (PiProtocol) Events() []string      { return piEvents }
func (PiProtocol) UsesCwdFallback() bool { return true }

func (PiProtocol) Normalize(event string, payload []byte) (NormalizedEvent, bool) {
    var p struct {
        ToolName string `json:"tool_name"`
        Message  string `json:"message"`
    }
    _ = json.Unmarshal(payload, &p)

    switch event {
    case "session_start":    return NormalizedEvent{StateEvent: state.EvSessionStart, SoundEvent: "session_start"}, true
    case "input":            return NormalizedEvent{StateEvent: state.EvUserResume}, true
    case "tool_call":        return NormalizedEvent{StateEvent: state.EvPreToolUse, ToolName: p.ToolName}, true
    case "tool_result":      return NormalizedEvent{StateEvent: state.EvPostToolUse, ToolName: p.ToolName}, true
    case "agent_end":        return NormalizedEvent{StateEvent: state.EvStop, SoundEvent: "session_idle"}, true
    case "session_shutdown": return NormalizedEvent{StateEvent: state.EvSessionEnd, SoundEvent: "session_completed"}, true
    }
    return NormalizedEvent{}, false
}
```

### 5.2 TypeScript extension

`PiProtocol.Install()` writes (or overwrites with `--force`) `~/.pi/agent/extensions/cleo.ts`. The content is embedded in `pi.go` as a string constant so `cleo doctor` can diff the on-disk file against it.

```typescript
// Generated by `cleo init` — do not edit manually. Re-run `cleo init` to update.
export default function(pi) {
  const hook = (event, ctx, extra = {}) => {
    const payload = JSON.stringify({ cwd: ctx.cwd, ...extra });
    pi.exec("cleo", ["hook", "pi", event, "--payload", payload], {});
  };

  pi.on("session_start",    (_, ctx) => hook("session_start",    ctx));
  pi.on("input",            (_, ctx) => hook("input",            ctx));
  pi.on("tool_call",        (e, ctx) => hook("tool_call",        ctx, { tool_name: e.toolName }));
  pi.on("tool_result",      (e, ctx) => hook("tool_result",      ctx, { tool_name: e.toolName }));
  pi.on("agent_end",        (_, ctx) => hook("agent_end",        ctx));
  pi.on("session_shutdown", (_, ctx) => hook("session_shutdown", ctx));
}
```

`PiProtocol.Cleanup()` removes the file if its content matches the embedded template. If the file has been user-modified, print a warning and leave it.

`ExpectedPiEntry()` returns the embedded template string for use by `cleo doctor`.

### 5.3 `--payload` flag on `cleo hook`

`pi.exec()` has no stdin parameter, so Pi can't pipe JSON the way Claude/Codex do. The `cleo hook` command gains a `--payload` flag; when set it takes precedence over stdin. Claude and Codex are unaffected — they continue to use stdin.

```go
// internal/cli/hook.go
var payloadFlag string
cmd.Flags().StringVar(&payloadFlag, "payload", "", "JSON payload (alternative to stdin, used by Pi extension)")

// In RunE — before passing body to hooks.Handle:
var body []byte
if payloadFlag != "" {
    body = []byte(payloadFlag)
} else {
    body, _ = io.ReadAll(os.Stdin)
}
```

### 5.4 `cleo init` — Pi added to multi-select

```
Which hook systems would you like to install?
> [x] Claude Code  (~/.claude/settings.json)
  [x] Codex        (~/.codex/hooks.json)
  [ ] Pi           (~/.pi/agent/extensions/cleo.ts)
```

### 5.5 `cleo doctor` — Pi check

Three checks, matching the Claude/Codex pattern:
- Extension file exists at `~/.pi/agent/extensions/cleo.ts`.
- Content matches the embedded template (diff if not).
- At least one Pi hook trace in the last 24 h (activity check).

---

## 6. `cleo init` Output Formatting Pass

`printInitSummary()` in `internal/cli/init.go` is rewritten using lipgloss (already a project dependency). No logic change — pure presentation.

**Before (current):**
```
Cleo hooks initialized

Installed:
  - Claude Code
    hooks: /Users/dhruvsaxena/.claude/settings.json
    events:
      - SessionStart
      - UserPromptSubmit
      ...
```

**After:**
```
✓ Cleo hooks initialized

  Claude Code
  ✓ hooks      ~/.claude/settings.json
  ✓ 8 events   SessionStart · UserPromptSubmit · PreToolUse · PostToolUse
               Notification · Stop · SessionEnd · SubagentStop

  Codex
  ✓ hooks      ~/.codex/hooks.json
  ✓ feature    ~/.codex/config.toml  ([features].hooks = true)
  ✓ 6 events   SessionStart · UserPromptSubmit · PreToolUse · PostToolUse
               PermissionRequest · Stop

  Pi
  ✓ extension  ~/.pi/agent/extensions/cleo.ts
  ✓ 6 events   session_start · input · tool_call · tool_result
               agent_end · session_shutdown

⚠  Codex requires manual hook approval
   Open Codex, run /hooks, and approve entries starting with:
   /Users/dhruvsaxena/.local/bin/cleo hook codex
   Restart any open Codex sessions first so they pick up ~/.codex/config.toml.
```

Styling:
- Section header (`✓ Cleo hooks initialized`) — bold green
- Per-agent name — bold, default colour
- `✓` labels (`hooks`, `feature`, `extension`, `events`) — green
- File paths — dimmed
- Event names — regular weight, dot-separated, wrapped to two rows
- `⚠` block — yellow; only printed when Codex or another agent requires a manual step

Lipgloss handles `NO_COLOR` and non-TTY detection automatically — piped output degrades to plain text.

---

## 7. File Impact

| File | Change |
|---|---|
| `internal/hooks/protocol.go` | NEW — `Protocol` interface, `NormalizedEvent`, `Protocols()`, `applyNormalized()`, `findProtocol()` |
| `internal/hooks/claude.go` | NEW — `ClaudeProtocol` struct (logic extracted from `handler.go`) |
| `internal/hooks/codex.go` | NEW — `CodexProtocol` struct + `synthesizeNotification()` |
| `internal/hooks/pi.go` | NEW — `PiProtocol` struct, TS template constant, `InstallPi()`, `ExpectedPiEntry()` |
| `internal/hooks/handler.go` | CHANGE — `Handle()` uses interface; `resolveSession()` + `applyNormalized()` extracted; `handleClaude`/`handleCodex` deleted |
| `internal/hooks/install.go` | CHANGE — `ClaudeProtocol.Install/Cleanup` + `CodexProtocol.Install/Cleanup` delegate to existing helpers; `ExpectedPiEntry()` added for doctor |
| `internal/hooks/handler_test.go` | CHANGE — tests updated; one test per `Normalize()` mapping per protocol |
| `internal/hooks/install_test.go` | CHANGE — Pi install test: golden file comparison of generated TS |
| `internal/cli/hook.go` | CHANGE — `--payload` flag |
| `internal/cli/init.go` | CHANGE — Pi added to multi-select; `printInitSummary` rewritten with lipgloss |
| `internal/cli/doctor.go` | CHANGE — Pi check (file exists + content diff + activity) |
| `internal/cli/init_test.go` | NEW — snapshot test for formatted `printInitSummary` output |

---

## 8. Testing

| Area | Unit tests | Manual smoke |
|---|---|---|
| `ClaudeProtocol.Normalize` | One test per event including `SubagentStop` log-only | — |
| `CodexProtocol.Normalize` | `PermissionRequest` → Notification synthesis; shared events delegate to Claude | — |
| `PiProtocol.Normalize` | One test per pi event; unknown event returns `ok=false` | — |
| `synthesizeNotification` | Three cases: command present, description present, fallback to tool_name | — |
| `resolveSession` | `UsesCwdFallback=false` → `FindByCwd` never called; `UsesCwdFallback=true` → called on env miss | — |
| `PiProtocol.Install` | Golden file: generated TS matches embedded template | `cleo init` → verify `~/.pi/agent/extensions/cleo.ts` written |
| `cleo doctor` Pi | Fixture: file missing, file stale, file current | Run `cleo doctor` against real `~/.pi` dir |
| `printInitSummary` | Snapshot test of lipgloss output (strip ANSI for comparison) | `cleo init --yes` visual inspection |
| `--payload` flag | `payload` flag overrides stdin; stdin still works when flag absent | `echo '{}' \| cleo hook claude SessionStart` still works |

---

## 9. Out of Scope

- OpenRouter support (separate spec; depends on which CLI client the user runs)
- Pi `WaitingForInput` state (Pi has no native notification event; deferred until Pi exposes one)
- Worktree-aware session resolution
- Plugin/dynamic protocol loading (three hardcoded protocols is fine for now)
