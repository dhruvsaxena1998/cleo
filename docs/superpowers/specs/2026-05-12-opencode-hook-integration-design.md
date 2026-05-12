# OpenCode Hook Integration — Design Spec

## Problem

Opencode is already a registered agent in cleo (`defaults.go`) but its `Hooks` field is `"none"`. No hook protocol is wired up, so cleo cannot track opencode session lifecycle: no TUI state transitions, no sound events, no `cleo doctor` coverage.

## Approach

Opencode's hook system is TypeScript-plugin-based. Plugins in `~/.config/opencode/plugins/` are auto-loaded at startup. Cleo ships a generated `cleo.ts` plugin (the same pattern as Pi's `~/.pi/agent/extensions/cleo.ts`) that calls `cleo hook opencode <event> --payload <json>` for each relevant lifecycle event.

## Event Mapping

| opencode event        | cleo state event  | sound              |
|-----------------------|-------------------|--------------------|
| `session.created`     | `EvSessionStart`  | `session_start`    |
| `tool.execute.before` | `EvPreToolUse`    | —                  |
| `tool.execute.after`  | `EvPostToolUse`   | —                  |
| `permission.asked`    | `EvNotification`  | `needs_input`      |
| `session.idle`        | `EvStop`          | `session_idle`     |
| `session.deleted`     | `EvSessionEnd`    | `session_completed`|
| `session.error`       | `EvError`         | `session_error`    |

`permission.asked` sets `SuppressWhenIdle: false` (genuine block, not idle nudge).

## Session ID Resolution

The opencode plugin uses Bun's `$` shell, which inherits the parent process environment including `CLEO_SESSION_ID` (set by cleo when spawning the tmux session). Each hook payload also includes `cwd: directory` for the cwd-fallback path, and `session_id: sessionID` from the event callback for future extensibility. `UsesCwdFallback()` returns `true` (defensive, mirrors Pi).

## Components

### `internal/hooks/opencode.go` (new)
- `openCodePluginsDir` package-level var (overrideable in tests)
- `openCodePlugDir()` helper — returns `~/.config/opencode/plugins`
- `openCodePluginTemplate` const — TypeScript plugin content
- `OpenCodeProtocol` struct implementing `Protocol`
- `ExpectedOpenCodeEntry()` — returns template string (for doctor diff)

### `internal/hooks/protocol.go` (modify)
- Add `OpenCodeProtocol{}` to `Protocols()`

### `internal/config/defaults.go` (modify)
- Change opencode agent's `Hooks: "none"` → `Hooks: "opencode"`

### `internal/cli/init.go` (modify)
- Add `hookOpenCode = "opencode"` constant
- Add opencode option to `promptHookSelection` (default: no, like Pi)
- Add `case hookOpenCode` in install switch

### `internal/cli/doctor.go` (modify)
- Add `OpenCodePluginPath string` to `doctorReport`
- Add `checkOpenCodeExtension(path string) doctorCheck`
- Add opencode plugin check + opencode activity trace to `diagnoseHooks`
- Update `newDoctorCmd` to pass opencode plugin path

## Testing Strategy

Each new function in `opencode.go` gets table-driven unit tests mirroring `pi_test.go`. Changes to `init.go` and `doctor.go` are covered by updating existing table tests and adding new assertions.

## Files Changed

| File | Action |
|---|---|
| `internal/hooks/opencode.go` | Create |
| `internal/hooks/opencode_test.go` | Create |
| `internal/hooks/protocol.go` | Modify (1 line) |
| `internal/config/defaults.go` | Modify (1 line) |
| `internal/cli/init.go` | Modify |
| `internal/cli/init_test.go` | Modify |
| `internal/cli/doctor.go` | Modify |
| `internal/cli/doctor_test.go` | Modify |
