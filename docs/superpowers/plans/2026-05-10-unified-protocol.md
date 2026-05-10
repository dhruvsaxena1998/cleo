# Unified Protocol Interface Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Introduce a `Protocol` interface and `NormalizedEvent` type so any coding agent can be added to cleo by implementing one interface, then add Pi as the first new protocol on top of it.

**Architecture:** Each agent protocol lives in its own file (`claude.go`, `codex.go`, `pi.go`) implementing the `Protocol` interface. `Handle()` looks up the protocol, resolves the session, calls `proto.Normalize()`, and passes the resulting `NormalizedEvent` to `applyNormalized()` — no more switches on protocol name in the handler. Pi uses a TypeScript extension that calls `cleo hook pi <event> --payload <json>` via `pi.exec()`.

**Tech Stack:** Go, lipgloss (charmbracelet/lipgloss — already a dependency), TypeScript (generated extension template embedded as a Go string constant).

**Spec:** `docs/superpowers/specs/2026-05-10-unified-protocol-design.md`

---

## File Map

| File | Action | Responsibility |
|---|---|---|
| `internal/hooks/protocol.go` | CREATE | `NormalizedEvent`, `Protocol` interface, `Protocols()`, `findProtocol()` |
| `internal/hooks/claude.go` | CREATE | `ClaudeProtocol` struct implementing `Protocol` |
| `internal/hooks/claude_test.go` | CREATE | `Normalize()` tests for every Claude event |
| `internal/hooks/codex.go` | CREATE | `CodexProtocol` struct + `synthesizeNotification()` |
| `internal/hooks/codex_test.go` | CREATE | `Normalize()` tests for Codex including `PermissionRequest` |
| `internal/hooks/pi.go` | CREATE | `PiProtocol` struct, TS template constant, `Install()`, `Cleanup()`, `ExpectedPiEntry()` |
| `internal/hooks/pi_test.go` | CREATE | `Normalize()` tests + golden-file `Install()` test |
| `internal/hooks/handler.go` | MODIFY | Rewrite `Handle()` using interface; extract `resolveSession()` + `applyNormalized()`; delete `handleClaude`/`handleCodex` |
| `internal/hooks/handler_test.go` | MODIFY | Update all call sites from `(stdin, stdout)` to `(payload []byte)`; add `resolveSession` cwd-fallback test |
| `internal/cli/hook.go` | MODIFY | Add `--payload` flag; read body from flag or stdin; drop `stdout` arg from `Handle()` call |
| `internal/cli/init.go` | MODIFY | Add Pi to multi-select; call through `Protocol.Install()`; rewrite `printInitSummary` with lipgloss |
| `internal/cli/init_test.go` | CREATE | Snapshot test for formatted `printInitSummary` output |
| `internal/cli/doctor.go` | MODIFY | Add Pi check (extension file exists + content diff) |
| `internal/cli/doctor_test.go` | MODIFY | Fixture tests for Pi check |

---

## Task 1: Define `NormalizedEvent` and `Protocol` interface

**Files:**
- Create: `internal/hooks/protocol.go`

This task creates the shared types. No tests — the interface is validated by the compiler as protocols implement it.

- [ ] **Step 1: Create `internal/hooks/protocol.go`**

```go
package hooks

import (
	"fmt"

	"github.com/dhruvsaxena1998/cleo/internal/state"
)

// NormalizedEvent is the canonical form every protocol produces after parsing
// its raw payload. Handle() consumes only this — no protocol-specific logic lives
// outside the Protocol implementation.
type NormalizedEvent struct {
	StateEvent state.Event // empty = no state transition
	SoundEvent string      // empty = no sound
	Message    string      // Notification / PermissionRequest text
	ToolName   string      // written to the event log
	LogOnly    bool        // log entry only, no state transition (e.g. SubagentStop)
	LogType    string      // events.Entry.Type override when LogOnly=true; defaults to string(StateEvent)
}

// Protocol describes a supported agent integration. Implement this interface to
// add a new agent — then add one line to Protocols() below.
type Protocol interface {
	// Name returns the identifier used in "cleo hook <protocol> <event>".
	Name() string
	// Events returns the hook event names this protocol subscribes to.
	Events() []string
	// Install writes hook config into the agent's config file(s).
	Install(cleoBin string, force bool) error
	// Cleanup removes cleo-owned hook entries from the agent's config file(s).
	Cleanup() error
	// Normalize converts a raw event name and JSON payload into a NormalizedEvent.
	// Returns ok=false if the event is unknown and should be silently ignored.
	Normalize(event string, payload []byte) (NormalizedEvent, bool)
	// UsesCwdFallback returns true when the protocol may not propagate
	// CLEO_SESSION_ID to hook subprocesses. resolveSession() calls FindByCwd
	// only when this is true.
	UsesCwdFallback() bool
}

// Protocols returns the registered set of supported agents.
// Adding a new agent: implement Protocol and add one line here.
func Protocols() []Protocol {
	return []Protocol{
		ClaudeProtocol{},
		CodexProtocol{},
		PiProtocol{},
	}
}

func findProtocol(protos []Protocol, name string) (Protocol, bool) {
	for _, p := range protos {
		if p.Name() == name {
			return p, true
		}
	}
	return nil, false
}

// protocolNames returns sorted names for error messages.
func protocolNames(protos []Protocol) []string {
	names := make([]string, len(protos))
	for i, p := range protos {
		names[i] = p.Name()
	}
	return names
}

var errUnknownProtocol = func(name string) error {
	return fmt.Errorf("unknown protocol %q", name)
}
```

- [ ] **Step 2: Verify it compiles (stubs for the three structs needed)**

Add temporary stub declarations at the bottom of `protocol.go` so the compiler is satisfied before the individual files exist. Remove these once Tasks 2–5 add the real structs.

```go
// Temporary compile stubs — remove once claude.go, codex.go, pi.go exist.
type ClaudeProtocol struct{}
type CodexProtocol struct{}
type PiProtocol struct{}
```

Run:
```bash
cd /path/to/cleo && go build ./internal/hooks/...
```
Expected: builds without errors.

- [ ] **Step 3: Commit**

```bash
git add internal/hooks/protocol.go
git commit -m "feat(hooks): add Protocol interface and NormalizedEvent type"
```

---

## Task 2: Implement `ClaudeProtocol` with TDD

**Files:**
- Create: `internal/hooks/claude_test.go`
- Create: `internal/hooks/claude.go`
- Modify: `internal/hooks/protocol.go` (remove Claude stub)

- [ ] **Step 1: Write the failing tests**

Create `internal/hooks/claude_test.go`:

```go
package hooks

import (
	"testing"

	"github.com/dhruvsaxena1998/cleo/internal/state"
)

func TestClaudeProtocol_Normalize(t *testing.T) {
	proto := ClaudeProtocol{}

	tests := []struct {
		event   string
		payload string
		want    NormalizedEvent
		wantOk  bool
	}{
		{
			event:   "SessionStart",
			payload: `{"tool_name":"bash"}`,
			want:    NormalizedEvent{StateEvent: state.EvSessionStart, SoundEvent: "session_start", ToolName: "bash"},
			wantOk:  true,
		},
		{
			event:   "UserPromptSubmit",
			payload: `{"tool_name":""}`,
			want:    NormalizedEvent{StateEvent: state.EvUserResume},
			wantOk:  true,
		},
		{
			event:   "PreToolUse",
			payload: `{"tool_name":"Bash"}`,
			want:    NormalizedEvent{StateEvent: state.EvPreToolUse, ToolName: "Bash"},
			wantOk:  true,
		},
		{
			event:   "PostToolUse",
			payload: `{"tool_name":"Bash"}`,
			want:    NormalizedEvent{StateEvent: state.EvPostToolUse, ToolName: "Bash"},
			wantOk:  true,
		},
		{
			event:   "Notification",
			payload: `{"tool_name":"Bash","message":"Approve command?"}`,
			want:    NormalizedEvent{StateEvent: state.EvNotification, SoundEvent: "needs_input", Message: "Approve command?", ToolName: "Bash"},
			wantOk:  true,
		},
		{
			event:   "Stop",
			payload: `{"tool_name":""}`,
			want:    NormalizedEvent{StateEvent: state.EvStop, SoundEvent: "session_idle"},
			wantOk:  true,
		},
		{
			event:   "SessionEnd",
			payload: `{"tool_name":""}`,
			want:    NormalizedEvent{StateEvent: state.EvSessionEnd, SoundEvent: "session_completed"},
			wantOk:  true,
		},
		{
			event:   "SubagentStop",
			payload: `{"tool_name":"mcp__tool"}`,
			want:    NormalizedEvent{LogOnly: true, LogType: "SubagentStop", ToolName: "mcp__tool"},
			wantOk:  true,
		},
		{
			event:   "UnknownEvent",
			payload: `{}`,
			wantOk:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.event, func(t *testing.T) {
			got, ok := proto.Normalize(tt.event, []byte(tt.payload))
			if ok != tt.wantOk {
				t.Fatalf("ok = %v, want %v", ok, tt.wantOk)
			}
			if ok && got != tt.want {
				t.Errorf("got  %+v\nwant %+v", got, tt.want)
			}
		})
	}
}

func TestClaudeProtocol_Metadata(t *testing.T) {
	proto := ClaudeProtocol{}
	if proto.Name() != "claude" {
		t.Errorf("Name() = %q, want \"claude\"", proto.Name())
	}
	if proto.UsesCwdFallback() {
		t.Error("Claude must not use cwd fallback")
	}
	if len(proto.Events()) == 0 {
		t.Error("Events() returned empty slice")
	}
}
```

- [ ] **Step 2: Run tests — expect failure**

```bash
go test ./internal/hooks/... -run TestClaudeProtocol -v
```
Expected: FAIL — `ClaudeProtocol` has no methods yet.

- [ ] **Step 3: Implement `claude.go`**

Create `internal/hooks/claude.go`:

```go
package hooks

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/dhruvsaxena1998/cleo/internal/state"
)

var claudeEvents = []string{
	"SessionStart", "UserPromptSubmit", "PreToolUse", "PostToolUse",
	"Notification", "Stop", "SessionEnd", "SubagentStop",
}

// ClaudeEvents returns the hook event names Claude Code fires.
func ClaudeEvents() []string { return append([]string(nil), claudeEvents...) }

type ClaudeProtocol struct{}

func (ClaudeProtocol) Name() string          { return "claude" }
func (ClaudeProtocol) Events() []string      { return ClaudeEvents() }
func (ClaudeProtocol) UsesCwdFallback() bool { return false }

func (ClaudeProtocol) Install(cleoBin string, force bool) error {
	home, _ := os.UserHomeDir()
	return InstallClaude(filepath.Join(home, ".claude", "settings.json"), cleoBin, force)
}

func (ClaudeProtocol) Cleanup() error {
	home, _ := os.UserHomeDir()
	_, err := CleanupClaude(filepath.Join(home, ".claude", "settings.json"))
	return err
}

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

- [ ] **Step 4: Remove the Claude stub from `protocol.go`**

Delete the line:
```go
type ClaudeProtocol struct{}
```
from the temporary stubs block at the bottom of `protocol.go`.

- [ ] **Step 5: Run tests — expect pass**

```bash
go test ./internal/hooks/... -run TestClaudeProtocol -v
```
Expected: PASS.

- [ ] **Step 6: Run full test suite**

```bash
go test ./...
```
Expected: all pass (handler_test.go still compiles because ClaudeProtocol struct still exists).

- [ ] **Step 7: Commit**

```bash
git add internal/hooks/claude.go internal/hooks/claude_test.go internal/hooks/protocol.go
git commit -m "feat(hooks): add ClaudeProtocol implementing Protocol interface"
```

---

## Task 3: Implement `CodexProtocol` with TDD

**Files:**
- Create: `internal/hooks/codex_test.go`
- Create: `internal/hooks/codex.go`
- Modify: `internal/hooks/protocol.go` (remove Codex stub)

- [ ] **Step 1: Write the failing tests**

Create `internal/hooks/codex_test.go`:

```go
package hooks

import (
	"testing"

	"github.com/dhruvsaxena1998/cleo/internal/state"
)

func TestCodexProtocol_PermissionRequest(t *testing.T) {
	proto := CodexProtocol{}

	tests := []struct {
		name    string
		payload string
		wantMsg string
	}{
		{
			name:    "command present",
			payload: `{"tool_name":"Bash","tool_input":{"command":"rm -rf /tmp/foo"}}`,
			wantMsg: "rm -rf /tmp/foo",
		},
		{
			name:    "description present, no command",
			payload: `{"tool_name":"Bash","tool_input":{"description":"delete temp files"}}`,
			wantMsg: "delete temp files",
		},
		{
			name:    "fallback to tool_name",
			payload: `{"tool_name":"Bash","tool_input":{}}`,
			wantMsg: "Bash",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := proto.Normalize("PermissionRequest", []byte(tt.payload))
			if !ok {
				t.Fatal("expected ok=true for PermissionRequest")
			}
			if got.StateEvent != state.EvNotification {
				t.Errorf("StateEvent = %q, want %q", got.StateEvent, state.EvNotification)
			}
			if got.SoundEvent != "needs_input" {
				t.Errorf("SoundEvent = %q, want \"needs_input\"", got.SoundEvent)
			}
			if got.Message != tt.wantMsg {
				t.Errorf("Message = %q, want %q", got.Message, tt.wantMsg)
			}
		})
	}
}

func TestCodexProtocol_SharedEvents(t *testing.T) {
	proto := CodexProtocol{}

	// Events shared with Claude delegate to ClaudeProtocol.Normalize.
	got, ok := proto.Normalize("PreToolUse", []byte(`{"tool_name":"Bash"}`))
	if !ok {
		t.Fatal("expected ok=true for PreToolUse")
	}
	if got.StateEvent != state.EvPreToolUse {
		t.Errorf("StateEvent = %q, want %q", got.StateEvent, state.EvPreToolUse)
	}
}

func TestCodexProtocol_UnknownEventIgnored(t *testing.T) {
	proto := CodexProtocol{}
	_, ok := proto.Normalize("UnknownEvent", []byte(`{}`))
	if ok {
		t.Error("expected ok=false for unknown event")
	}
}

func TestCodexProtocol_Metadata(t *testing.T) {
	proto := CodexProtocol{}
	if proto.Name() != "codex" {
		t.Errorf("Name() = %q, want \"codex\"", proto.Name())
	}
	if !proto.UsesCwdFallback() {
		t.Error("Codex must use cwd fallback")
	}
}

func TestSynthesizeNotification(t *testing.T) {
	tests := []struct {
		name    string
		payload string
		wantMsg string
	}{
		{"command wins over description", `{"tool_name":"T","tool_input":{"command":"cmd","description":"desc"}}`, "cmd"},
		{"description when no command", `{"tool_name":"T","tool_input":{"description":"desc"}}`, "desc"},
		{"tool_name as last resort", `{"tool_name":"T","tool_input":{}}`, "T"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var p struct {
				ToolName string `json:"tool_name"`
				Message  string `json:"message"`
			}
			result := synthesizeNotification([]byte(tt.payload))
			_ = json.Unmarshal(result, &p) // json imported in package via handler.go
			if p.Message != tt.wantMsg {
				t.Errorf("message = %q, want %q", p.Message, tt.wantMsg)
			}
		})
	}
}
```

Note: `json` is already imported in the `hooks` package via other files, so the `json.Unmarshal` call in the test compiles without an explicit import in the test file. If the compiler complains, add `"encoding/json"` to the test file imports.

- [ ] **Step 2: Run tests — expect failure**

```bash
go test ./internal/hooks/... -run "TestCodex|TestSynthesize" -v
```
Expected: FAIL — `CodexProtocol` has no methods, `synthesizeNotification` doesn't exist.

- [ ] **Step 3: Implement `codex.go`**

Create `internal/hooks/codex.go`:

```go
package hooks

import (
	"encoding/json"
	"os"
	"path/filepath"
)

var codexEvents = []string{
	"SessionStart", "UserPromptSubmit", "PreToolUse", "PostToolUse",
	"PermissionRequest", "Stop",
}

// CodexEvents returns the hook event names Codex fires.
func CodexEvents() []string { return append([]string(nil), codexEvents...) }

type CodexProtocol struct{}

func (CodexProtocol) Name() string          { return "codex" }
func (CodexProtocol) Events() []string      { return CodexEvents() }
func (CodexProtocol) UsesCwdFallback() bool { return true }

func (CodexProtocol) Install(cleoBin string, force bool) error {
	home, _ := os.UserHomeDir()
	return InstallCodex(
		filepath.Join(home, ".codex", "hooks.json"),
		filepath.Join(home, ".codex", "config.toml"),
		cleoBin, force,
	)
}

func (CodexProtocol) Cleanup() error {
	home, _ := os.UserHomeDir()
	_, err := CleanupCodex(filepath.Join(home, ".codex", "hooks.json"))
	return err
}

func (CodexProtocol) Normalize(event string, payload []byte) (NormalizedEvent, bool) {
	if event == "PermissionRequest" {
		return ClaudeProtocol{}.Normalize("Notification", synthesizeNotification(payload))
	}
	return ClaudeProtocol{}.Normalize(event, payload)
}

// synthesizeNotification builds a claudePayload-shaped JSON from a Codex
// PermissionRequest payload. Preference order: command > description > tool_name.
func synthesizeNotification(payload []byte) []byte {
	var pr struct {
		ToolName  string `json:"tool_name"`
		ToolInput struct {
			Command     string `json:"command"`
			Description string `json:"description"`
		} `json:"tool_input"`
	}
	_ = json.Unmarshal(payload, &pr)
	msg := pr.ToolName
	if pr.ToolInput.Command != "" {
		msg = pr.ToolInput.Command
	} else if pr.ToolInput.Description != "" {
		msg = pr.ToolInput.Description
	}
	out, _ := json.Marshal(struct {
		ToolName string `json:"tool_name"`
		Message  string `json:"message"`
	}{pr.ToolName, msg})
	return out
}
```

- [ ] **Step 4: Remove the Codex stub from `protocol.go`**

Delete the line:
```go
type CodexProtocol struct{}
```
from the temporary stubs block at the bottom of `protocol.go`.

- [ ] **Step 5: Run tests — expect pass**

```bash
go test ./internal/hooks/... -run "TestCodex|TestSynthesize" -v
```
Expected: PASS.

- [ ] **Step 6: Run full test suite**

```bash
go test ./...
```
Expected: all pass.

- [ ] **Step 7: Commit**

```bash
git add internal/hooks/codex.go internal/hooks/codex_test.go internal/hooks/protocol.go
git commit -m "feat(hooks): add CodexProtocol implementing Protocol interface"
```

---

## Task 4: Refactor `handler.go` + update `hook.go` with `--payload` flag

This is the largest task. It rewrites `Handle()` to use the Protocol interface, changes the function signature, extracts `resolveSession()` and `applyNormalized()`, and adds `--payload` to the CLI. The existing test suite is updated in the same commit to keep the repo always-green.

**Files:**
- Modify: `internal/hooks/handler.go`
- Modify: `internal/hooks/handler_test.go`
- Modify: `internal/cli/hook.go`

- [ ] **Step 1: Rewrite `internal/hooks/handler.go`**

Replace the entire file with:

```go
package hooks

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/dhruvsaxena1998/cleo/internal/events"
	"github.com/dhruvsaxena1998/cleo/internal/paths"
	"github.com/dhruvsaxena1998/cleo/internal/state"
)

type Player interface {
	Play(file string) error
	Available() bool
}

type Deps struct {
	Paths  paths.Paths
	State  *state.Store
	Config config.Config
	Events  func(sid string) *events.Log
	Sound   Player
	Focused func(sid string) bool
	// Now returns the CLEO_SESSION_ID from env. Replaced in tests.
	Now func() (string, error)
	// FindByCwd is the cwd-based fallback for protocols where env propagation
	// is not guaranteed (Codex, Pi). Given cwd and agent name, returns session ID.
	FindByCwd func(cwd, agent string) (string, error)
}

func DefaultNow() (string, error) {
	sid := os.Getenv("CLEO_SESSION_ID")
	if sid == "" {
		return "", errNoSession
	}
	return sid, nil
}

var errNoSession = fmt.Errorf("CLEO_SESSION_ID not set")

// Handle dispatches a hook event from any supported protocol.
// payload is the raw JSON body — from stdin (Claude/Codex) or --payload flag (Pi).
func Handle(d Deps, protocol, event string, payload []byte) error {
	proto, ok := findProtocol(Protocols(), protocol)
	if !ok {
		err := errUnknownProtocol(protocol)
		logHookErr(d.Paths, protocol, event, err)
		return err
	}

	sid := resolveSession(d, proto, payload)
	if sid == "" {
		return nil
	}

	norm, ok := proto.Normalize(event, payload)
	if !ok {
		return nil // unknown event for this protocol; silently ignore
	}

	return applyNormalized(d, sid, norm)
}

// resolveSession finds the cleo session ID for an incoming hook.
// Strategy A: CLEO_SESSION_ID env var (all protocols).
// Strategy B: cwd lookup (only when proto.UsesCwdFallback() is true).
func resolveSession(d Deps, proto Protocol, payload []byte) string {
	trace := hookTrace{Protocol: proto.Name(), EnvSession: os.Getenv("CLEO_SESSION_ID") != ""}

	sid, err := d.Now()
	if err == nil {
		if d.State != nil {
			if _, sErr := d.State.Get(sid); sErr != nil {
				trace.FallbackReason = "env_unknown_session"
				err = sErr
				sid = ""
			} else {
				trace.FallbackReason = "env_present"
				trace.ResolvedSession = sid
			}
		} else {
			trace.FallbackReason = "env_present"
			trace.ResolvedSession = sid
		}
	} else {
		trace.FallbackReason = "env_missing"
	}

	if (err != nil || sid == "") && proto.UsesCwdFallback() && d.FindByCwd != nil {
		var base struct {
			Cwd string `json:"cwd"`
		}
		_ = json.Unmarshal(payload, &base)
		trace.Cwd = base.Cwd
		if base.Cwd == "" {
			if wd, wdErr := os.Getwd(); wdErr == nil {
				base.Cwd = wd
				trace.Cwd = wd
			}
		}
		if base.Cwd != "" {
			resolved, fbErr := d.FindByCwd(base.Cwd, proto.Name())
			if fbErr != nil || resolved == "" {
				trace.FallbackReason = "no_match"
				err = fbErr
			} else {
				trace.ResolvedSession = resolved
				sid = resolved
				err = nil
			}
		}
	}

	if err != nil || sid == "" {
		trace.Result = "ignored:no_session"
		logHookTrace(d.Paths, trace)
		if trace.FallbackReason == "no_match" {
			logHookErr(d.Paths, proto.Name(), "", fmt.Errorf("no session matched cwd=%q", trace.Cwd))
		}
		return ""
	}
	trace.Result = "resolved"
	logHookTrace(d.Paths, trace)
	return sid
}

// applyNormalized applies a NormalizedEvent to state, event log, and sound.
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

func sessionFocused(d Deps, sid string) bool {
	return d.Focused != nil && d.Focused(sid)
}

func playSound(d Deps, soundEvent string) {
	if !d.Sound.Available() {
		return
	}
	file := d.Config.Sound.Events[soundEvent]
	if file == "" {
		return
	}
	full := file
	if !filepath.IsAbs(full) {
		full = filepath.Join(d.Paths.SoundsDir(), file)
	}
	_ = d.Sound.Play(full)
}

type hookTrace struct {
	Protocol        string `json:"protocol"`
	Event           string `json:"event"`
	EnvSession      bool   `json:"env_session"`
	Cwd             string `json:"cwd,omitempty"`
	ResolvedSession string `json:"resolved_session,omitempty"`
	Result          string `json:"result"`
	FallbackReason  string `json:"fallback_reason,omitempty"`
}

func logHookTrace(p paths.Paths, trace hookTrace) {
	f, e := os.OpenFile(p.HookTraceLog(), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if e != nil {
		return
	}
	defer f.Close()
	row := struct {
		At string `json:"at"`
		hookTrace
	}{
		At:        time.Now().Format(time.RFC3339),
		hookTrace: trace,
	}
	b, _ := json.Marshal(row)
	fmt.Fprintln(f, string(b))
}

func logHookErr(p paths.Paths, protocol, event string, err error) {
	f, e := os.OpenFile(p.HookErrLog(), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if e != nil {
		return
	}
	defer f.Close()
	fmt.Fprintf(f, "%s [%s/%s] %v\n", time.Now().Format(time.RFC3339), protocol, event, err)
}
```

> **Note:** `Deps.Config` stays as `config.Config` (same as today). `playSound` accesses `d.Config.Sound.Events[soundEvent]` directly. Add `"github.com/dhruvsaxena1998/cleo/internal/config"` to the import block.

- [ ] **Step 2: Update `internal/hooks/handler_test.go`**

The `Handle()` signature changed from `(stdin io.Reader, stdout io.Writer)` to `(payload []byte)`. Update every call in the file. Here are the exact replacements:

**`setup()` function** — add missing `config` import and keep as-is (no signature change needed in setup itself).

**Every `Handle(deps, ..., strings.NewReader(...), &bytes.Buffer{})` call** becomes `Handle(deps, ..., []byte(...))`.

Full updated test file:

```go
package hooks

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/dhruvsaxena1998/cleo/internal/config"
	"github.com/dhruvsaxena1998/cleo/internal/events"
	"github.com/dhruvsaxena1998/cleo/internal/paths"
	"github.com/dhruvsaxena1998/cleo/internal/state"
)

func setup(t *testing.T) (Deps, *state.Store, paths.Paths) {
	root := t.TempDir()
	p := paths.NewWithRoot(root)
	st := state.NewStore(p.StateFile(), p.StateLock())
	_ = st.Put(state.Session{ID: "cleo-x-claude-1", Agent: "claude", State: state.Spawning})
	cfg, _ := config.Load(p.ConfigFile())
	deps := Deps{
		Paths:  p,
		State:  st,
		Config: cfg,
		Events: func(sid string) *events.Log { return events.NewLog(p.EventsLog(sid)) },
		Sound:  noopPlayer{},
		Now:    func() (string, error) { return "cleo-x-claude-1", nil },
	}
	return deps, st, p
}

type noopPlayer struct{}

func (noopPlayer) Play(string) error { return nil }
func (noopPlayer) Available() bool   { return false }

type recordingPlayer struct{ played []string }

func (p *recordingPlayer) Play(file string) error {
	p.played = append(p.played, file)
	return nil
}
func (*recordingPlayer) Available() bool { return true }

func TestClaudePreToolUseTransitions(t *testing.T) {
	deps, st, _ := setup(t)
	if err := Handle(deps, "claude", "PreToolUse", []byte(`{"tool_name":"Bash"}`)); err != nil {
		t.Fatal(err)
	}
	got, _ := st.Get("cleo-x-claude-1")
	if got.State != state.Running {
		t.Errorf("state %s", got.State)
	}
}

func TestClaudeNotificationSetsLastMessage(t *testing.T) {
	deps, st, _ := setup(t)
	_, _ = st.Apply("cleo-x-claude-1", state.EvSessionStart, "")
	if err := Handle(deps, "claude", "Notification", []byte(`{"message":"Approve Bash command?"}`)); err != nil {
		t.Fatal(err)
	}
	got, _ := st.Get("cleo-x-claude-1")
	if got.State != state.WaitingForInput {
		t.Errorf("state %s", got.State)
	}
	if got.LastMessage == "" {
		t.Errorf("last message empty")
	}
}

func TestDisabledSoundEventDoesNotPlay(t *testing.T) {
	deps, st, _ := setup(t)
	player := &recordingPlayer{}
	deps.Sound = player
	deps.Config.Sound.EventEnabled["session_completed"] = false
	_, _ = st.Apply("cleo-x-claude-1", state.EvSessionStart, "")
	if err := Handle(deps, "claude", "SessionEnd", []byte(`{}`)); err != nil {
		t.Fatal(err)
	}
	if len(player.played) != 0 {
		t.Errorf("expected no sound, played %v", player.played)
	}
}

func TestFocusedSessionDoesNotPlaySound(t *testing.T) {
	deps, st, _ := setup(t)
	player := &recordingPlayer{}
	deps.Sound = player
	deps.Focused = func(sid string) bool { return sid == "cleo-x-claude-1" }
	_, _ = st.Apply("cleo-x-claude-1", state.EvSessionStart, "")
	if err := Handle(deps, "claude", "SessionEnd", []byte(`{}`)); err != nil {
		t.Fatal(err)
	}
	if len(player.played) != 0 {
		t.Errorf("expected no sound for focused session, played %v", player.played)
	}
}

func TestEnabledSoundEventPlays(t *testing.T) {
	deps, st, _ := setup(t)
	player := &recordingPlayer{}
	deps.Sound = player
	_, _ = st.Apply("cleo-x-claude-1", state.EvSessionStart, "")
	if err := Handle(deps, "claude", "SessionEnd", []byte(`{}`)); err != nil {
		t.Fatal(err)
	}
	if len(player.played) != 1 {
		t.Fatalf("expected one sound, played %v", player.played)
	}
	if !strings.HasSuffix(player.played[0], "done.wav") {
		t.Errorf("expected done.wav, got %q", player.played[0])
	}
}

func TestClaudeUserPromptSubmitResumesRunning(t *testing.T) {
	deps, st, _ := setup(t)
	_, _ = st.Apply("cleo-x-claude-1", state.EvSessionStart, "")
	_, _ = st.Apply("cleo-x-claude-1", state.EvStop, "")
	if err := Handle(deps, "claude", "UserPromptSubmit", []byte(`{"cwd":"/tmp/myproject"}`)); err != nil {
		t.Fatal(err)
	}
	got, _ := st.Get("cleo-x-claude-1")
	if got.State != state.Running {
		t.Errorf("expected running after prompt submit, got %s", got.State)
	}
}

func TestClaudeStandaloneSessionIgnoredWhenNoEnvVar(t *testing.T) {
	deps, st, _ := setup(t)
	_, _ = st.Apply("cleo-x-claude-1", state.EvSessionStart, "")
	deps.Now = func() (string, error) { return "", fmt.Errorf("not set") }
	deps.FindByCwd = func(cwd, agent string) (string, error) {
		t.Errorf("FindByCwd must not be called for claude protocol")
		return "cleo-x-claude-1", nil
	}
	_ = Handle(deps, "claude", "Stop", []byte(`{"cwd":"/tmp/myproject"}`))
	got, _ := st.Get("cleo-x-claude-1")
	if got.State != state.Running {
		t.Errorf("expected state unchanged (Running), got %s", got.State)
	}
}

func TestCodexPermissionRequestSetsWaitingForInput(t *testing.T) {
	deps, st, _ := setup(t)
	_, _ = st.Apply("cleo-x-claude-1", state.EvSessionStart, "")
	payload := []byte(`{"tool_name":"Bash","tool_input":{"command":"rm -rf /tmp/foo"}}`)
	if err := Handle(deps, "codex", "PermissionRequest", payload); err != nil {
		t.Fatal(err)
	}
	got, _ := st.Get("cleo-x-claude-1")
	if got.State != state.WaitingForInput {
		t.Errorf("expected waiting_for_input, got %s", got.State)
	}
	if got.LastMessage == "" {
		t.Errorf("last message should be set from tool input command")
	}
}

func TestCodexUserPromptSubmitResumesRunning(t *testing.T) {
	deps, st, _ := setup(t)
	_, _ = st.Apply("cleo-x-claude-1", state.EvSessionStart, "")
	_, _ = st.Apply("cleo-x-claude-1", state.EvStop, "")
	if err := Handle(deps, "codex", "UserPromptSubmit", []byte(`{"cwd":"/tmp/myproject"}`)); err != nil {
		t.Fatal(err)
	}
	got, _ := st.Get("cleo-x-claude-1")
	if got.State != state.Running {
		t.Errorf("expected running after prompt submit, got %s", got.State)
	}
}

func TestCodexCwdFallbackWhenNoEnvVar(t *testing.T) {
	deps, st, _ := setup(t)
	_, _ = st.Apply("cleo-x-claude-1", state.EvSessionStart, "")
	deps.Now = func() (string, error) { return "", fmt.Errorf("not set") }
	deps.FindByCwd = func(cwd, agent string) (string, error) {
		if cwd == "/tmp/myproject" && agent == "codex" {
			return "cleo-x-claude-1", nil
		}
		return "", nil
	}
	payload := []byte(`{"cwd":"/tmp/myproject","tool_name":"Bash"}`)
	if err := Handle(deps, "codex", "PreToolUse", payload); err != nil {
		t.Fatal(err)
	}
	got, _ := st.Get("cleo-x-claude-1")
	if got.State != state.Running {
		t.Errorf("expected running via cwd fallback, got %s", got.State)
	}
}

func TestCodexCwdFallbackUsesProcessWorkingDirectory(t *testing.T) {
	deps, st, _ := setup(t)
	_, _ = st.Apply("cleo-x-claude-1", state.EvSessionStart, "")
	deps.Now = func() (string, error) { return "", fmt.Errorf("not set") }
	deps.FindByCwd = func(cwd, agent string) (string, error) {
		if cwd != "" && agent == "codex" {
			return "cleo-x-claude-1", nil
		}
		return "", nil
	}
	_ = Handle(deps, "codex", "Stop", []byte(`{}`))
	got, _ := st.Get("cleo-x-claude-1")
	if got.State != state.Idle {
		t.Errorf("expected idle via process cwd fallback, got %s", got.State)
	}
}

func TestHandleUnknownProtocolLogsError(t *testing.T) {
	deps, _, p := setup(t)
	_ = Handle(deps, "unknown-proto", "SomeEvent", []byte(""))
	b, err := os.ReadFile(p.HookErrLog())
	if err != nil {
		t.Fatalf("hook-errors.log not created: %v", err)
	}
	if !strings.Contains(string(b), "unknown-proto") {
		t.Errorf("expected protocol name in error log, got: %s", string(b))
	}
}

func TestResolveSession_CwdFallbackNotCalledForClaude(t *testing.T) {
	deps, _, _ := setup(t)
	deps.Now = func() (string, error) { return "", fmt.Errorf("not set") }
	called := false
	deps.FindByCwd = func(cwd, agent string) (string, error) {
		called = true
		return "", nil
	}
	_ = Handle(deps, "claude", "PreToolUse", []byte(`{"cwd":"/proj"}`))
	if called {
		t.Error("FindByCwd must not be called for ClaudeProtocol (UsesCwdFallback=false)")
	}
}

func TestResolveSession_CwdFallbackCalledForPi(t *testing.T) {
	deps, st, _ := setup(t)
	_ = st.Put(state.Session{ID: "cleo-x-pi-1", Agent: "pi", State: state.Running})
	deps.Now = func() (string, error) { return "", fmt.Errorf("not set") }
	called := false
	deps.FindByCwd = func(cwd, agent string) (string, error) {
		called = true
		if agent == "pi" {
			return "cleo-x-pi-1", nil
		}
		return "", nil
	}
	_ = Handle(deps, "pi", "session_start", []byte(`{"cwd":"/proj"}`))
	if !called {
		t.Error("FindByCwd must be called for PiProtocol (UsesCwdFallback=true) when env absent")
	}
}

func TestFallbackReasonEnvPresent(t *testing.T) {
	d, _, p := setup(t)
	d.Now = func() (string, error) { return "cleo-x-claude-1", nil }
	_ = Handle(d, "claude", "PreToolUse", []byte(`{}`))
	row := lastTraceRow(t, p.HookTraceLog())
	if row.FallbackReason != "env_present" {
		t.Errorf("fallback_reason: want env_present, got %q", row.FallbackReason)
	}
}

func TestFallbackReasonEnvMissing(t *testing.T) {
	d, _, p := setup(t)
	d.Now = func() (string, error) { return "", errNoSessionTest }
	_ = Handle(d, "claude", "PreToolUse", []byte(`{}`))
	row := lastTraceRow(t, p.HookTraceLog())
	if row.FallbackReason != "env_missing" {
		t.Errorf("fallback_reason: want env_missing, got %q", row.FallbackReason)
	}
}

func TestFallbackReasonEnvUnknownSession(t *testing.T) {
	d, _, p := setup(t)
	d.Now = func() (string, error) { return "stale-sid", nil }
	_ = Handle(d, "claude", "PreToolUse", []byte(`{}`))
	row := lastTraceRow(t, p.HookTraceLog())
	if row.FallbackReason != "env_unknown_session" {
		t.Errorf("fallback_reason: want env_unknown_session, got %q", row.FallbackReason)
	}
}

func TestFallbackReasonNoMatchCodex(t *testing.T) {
	d, _, p := setup(t)
	d.Now = func() (string, error) { return "", errNoSessionTest }
	d.FindByCwd = func(cwd, agent string) (string, error) {
		return "", os.ErrNotExist
	}
	_ = Handle(d, "codex", "PreToolUse", []byte(`{"cwd":"/some/path"}`))
	row := lastTraceRow(t, p.HookTraceLog())
	if row.FallbackReason != "no_match" {
		t.Errorf("fallback_reason: want no_match, got %q", row.FallbackReason)
	}
	errLog, err := os.ReadFile(p.HookErrLog())
	if err != nil {
		t.Fatalf("hook-errors.log not created: %v", err)
	}
	if !strings.Contains(string(errLog), "/some/path") {
		t.Errorf("expected cwd in error log, got: %s", string(errLog))
	}
}

type traceRowForTest struct {
	At              string `json:"at"`
	Protocol        string `json:"protocol"`
	Event           string `json:"event"`
	Cwd             string `json:"cwd"`
	EnvSession      bool   `json:"env_session"`
	ResolvedSession string `json:"resolved_session"`
	Result          string `json:"result"`
	FallbackReason  string `json:"fallback_reason"`
}

func lastTraceRow(t *testing.T, path string) traceRowForTest {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read trace: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) == 0 {
		t.Fatalf("no trace rows at %s", path)
	}
	var row traceRowForTest
	if err := json.Unmarshal([]byte(lines[len(lines)-1]), &row); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	return row
}

var errNoSessionTest = errors.New("no session")
```

> **Note:** `p.HookTraceRow()` in `TestFallbackReasonEnvUnknownSession` — check the actual method name on `paths.Paths`; it may be `p.HookTraceLog()`. Use whichever exists.

- [ ] **Step 3: Update `internal/cli/hook.go`**

Replace the `RunE` body to add `--payload` and pass `[]byte` to `Handle`:

```go
package cli

import (
	"io"
	"os"

	"github.com/spf13/cobra"

	"github.com/dhruvsaxena1998/cleo/internal/hooks"
	"github.com/dhruvsaxena1998/cleo/internal/state"
)

func newHookCmd(getCtx func() *Ctx) *cobra.Command {
	var payloadFlag string

	cmd := &cobra.Command{
		Use:    "hook <protocol> <event>",
		Short:  "Internal: invoked by hook configs",
		Args:   cobra.ExactArgs(2),
		Hidden: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			c := getCtx()

			// Read payload: --payload flag takes precedence over stdin.
			// Pi uses --payload because pi.exec() has no stdin support.
			// Claude and Codex pipe JSON via stdin.
			var body []byte
			if payloadFlag != "" {
				body = []byte(payloadFlag)
			} else {
				body, _ = io.ReadAll(os.Stdin)
			}

			deps := hooks.Deps{
				Paths:  c.Paths,
				State:  c.State,
				Config: c.Config,
				Events: c.Events,
				Sound:  c.Player,
				Focused: func(sid string) bool {
					return c.Focus.IsFocused(sid)
				},
				Now: hooks.DefaultNow,
				FindByCwd: func(cwd, agent string) (string, error) {
					proj, err := c.Projects.ResolveFromCwd(cwd)
					if err != nil {
						return "", err
					}
					sessions, err := c.State.List()
					if err != nil {
						return "", err
					}
					var best state.Session
					for _, s := range sessions {
						if s.ProjectID == proj.ID && s.Agent == agent && !s.State.IsFinished() {
							if best.ID == "" || s.StartedAt.After(best.StartedAt) {
								best = s
							}
						}
					}
					if best.ID == "" {
						return "", nil
					}
					return best.ID, nil
				},
			}
			return hooks.Handle(deps, args[0], args[1], body)
		},
	}
	cmd.Flags().StringVar(&payloadFlag, "payload", "", "JSON payload (alternative to stdin, used by Pi extension)")
	return cmd
}
```

- [ ] **Step 4: Run the full test suite**

```bash
go test ./...
```
Expected: all pass. If there are compilation errors due to the `Deps.Config` interface change, revert to keeping `Deps.Config config.Config` (see note in Step 1).

- [ ] **Step 5: Commit**

```bash
git add internal/hooks/handler.go internal/hooks/handler_test.go internal/hooks/protocol.go internal/cli/hook.go
git commit -m "refactor(hooks): rewrite Handle() using Protocol interface; add --payload flag"
```

---

## Task 5: Implement `PiProtocol.Normalize()` with TDD

**Files:**
- Create: `internal/hooks/pi_test.go`
- Create: `internal/hooks/pi.go`
- Modify: `internal/hooks/protocol.go` (remove Pi stub)

- [ ] **Step 1: Write the failing tests**

Create `internal/hooks/pi_test.go`:

```go
package hooks

import (
	"testing"

	"github.com/dhruvsaxena1998/cleo/internal/state"
)

func TestPiProtocol_Normalize(t *testing.T) {
	proto := PiProtocol{}

	tests := []struct {
		event   string
		payload string
		want    NormalizedEvent
		wantOk  bool
	}{
		{
			event:   "session_start",
			payload: `{"cwd":"/proj"}`,
			want:    NormalizedEvent{StateEvent: state.EvSessionStart, SoundEvent: "session_start"},
			wantOk:  true,
		},
		{
			event:   "input",
			payload: `{"cwd":"/proj"}`,
			want:    NormalizedEvent{StateEvent: state.EvUserResume},
			wantOk:  true,
		},
		{
			event:   "tool_call",
			payload: `{"cwd":"/proj","tool_name":"bash"}`,
			want:    NormalizedEvent{StateEvent: state.EvPreToolUse, ToolName: "bash"},
			wantOk:  true,
		},
		{
			event:   "tool_result",
			payload: `{"cwd":"/proj","tool_name":"bash"}`,
			want:    NormalizedEvent{StateEvent: state.EvPostToolUse, ToolName: "bash"},
			wantOk:  true,
		},
		{
			event:   "agent_end",
			payload: `{"cwd":"/proj"}`,
			want:    NormalizedEvent{StateEvent: state.EvStop, SoundEvent: "session_idle"},
			wantOk:  true,
		},
		{
			event:   "session_shutdown",
			payload: `{"cwd":"/proj"}`,
			want:    NormalizedEvent{StateEvent: state.EvSessionEnd, SoundEvent: "session_completed"},
			wantOk:  true,
		},
		{
			event:   "unknown_event",
			payload: `{}`,
			wantOk:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.event, func(t *testing.T) {
			got, ok := proto.Normalize(tt.event, []byte(tt.payload))
			if ok != tt.wantOk {
				t.Fatalf("ok = %v, want %v", ok, tt.wantOk)
			}
			if ok && got != tt.want {
				t.Errorf("got  %+v\nwant %+v", got, tt.want)
			}
		})
	}
}

func TestPiProtocol_Metadata(t *testing.T) {
	proto := PiProtocol{}
	if proto.Name() != "pi" {
		t.Errorf("Name() = %q, want \"pi\"", proto.Name())
	}
	if !proto.UsesCwdFallback() {
		t.Error("Pi must use cwd fallback")
	}
	if len(proto.Events()) == 0 {
		t.Error("Events() returned empty slice")
	}
}
```

- [ ] **Step 2: Run tests — expect failure**

```bash
go test ./internal/hooks/... -run TestPiProtocol -v
```
Expected: FAIL — `PiProtocol` has no methods yet.

- [ ] **Step 3: Create `internal/hooks/pi.go` with `Normalize()` only**

```go
package hooks

import (
	"encoding/json"

	"github.com/dhruvsaxena1998/cleo/internal/state"
)

var piEvents = []string{
	"session_start", "input", "tool_call", "tool_result", "agent_end", "session_shutdown",
}

// PiEvents returns the lifecycle event names cleo subscribes to in Pi.
func PiEvents() []string { return append([]string(nil), piEvents...) }

type PiProtocol struct{}

func (PiProtocol) Name() string          { return "pi" }
func (PiProtocol) Events() []string      { return PiEvents() }
func (PiProtocol) UsesCwdFallback() bool { return true }

// Install and Cleanup are implemented in Task 6.
func (PiProtocol) Install(cleoBin string, force bool) error { return nil }
func (PiProtocol) Cleanup() error                           { return nil }

func (PiProtocol) Normalize(event string, payload []byte) (NormalizedEvent, bool) {
	var p struct {
		ToolName string `json:"tool_name"`
		Message  string `json:"message"`
	}
	_ = json.Unmarshal(payload, &p)

	switch event {
	case "session_start":
		return NormalizedEvent{StateEvent: state.EvSessionStart, SoundEvent: "session_start"}, true
	case "input":
		return NormalizedEvent{StateEvent: state.EvUserResume}, true
	case "tool_call":
		return NormalizedEvent{StateEvent: state.EvPreToolUse, ToolName: p.ToolName}, true
	case "tool_result":
		return NormalizedEvent{StateEvent: state.EvPostToolUse, ToolName: p.ToolName}, true
	case "agent_end":
		return NormalizedEvent{StateEvent: state.EvStop, SoundEvent: "session_idle"}, true
	case "session_shutdown":
		return NormalizedEvent{StateEvent: state.EvSessionEnd, SoundEvent: "session_completed"}, true
	}
	return NormalizedEvent{}, false
}
```

- [ ] **Step 4: Remove the Pi stub from `protocol.go`**

Delete the line:
```go
type PiProtocol struct{}
```
from the temporary stubs block (and remove the comment if the block is now empty).

- [ ] **Step 5: Run tests — expect pass**

```bash
go test ./internal/hooks/... -run TestPiProtocol -v
```
Expected: PASS.

- [ ] **Step 6: Run full test suite**

```bash
go test ./...
```
Expected: all pass.

- [ ] **Step 7: Commit**

```bash
git add internal/hooks/pi.go internal/hooks/pi_test.go internal/hooks/protocol.go
git commit -m "feat(hooks): add PiProtocol with Normalize() for all Pi lifecycle events"
```

---

## Task 6: Implement `PiProtocol.Install()`, `Cleanup()`, and `ExpectedPiEntry()`

**Files:**
- Modify: `internal/hooks/pi.go`
- Modify: `internal/hooks/pi_test.go`

- [ ] **Step 1: Write the failing install test**

Add to `internal/hooks/pi_test.go`:

```go
import (
	"os"
	"path/filepath"
	"testing"

	"github.com/dhruvsaxena1998/cleo/internal/state"
)

func TestPiProtocol_Install_WritesExtension(t *testing.T) {
	dir := t.TempDir()
	extDir := filepath.Join(dir, ".pi", "agent", "extensions")

	// Temporarily override the home-dir resolution by injecting the dir.
	// PiProtocol.Install uses os.UserHomeDir(); we patch via a test helper.
	// See implementation step for the piInstallDir variable trick.
	origDir := piExtensionsDir
	piExtensionsDir = extDir
	defer func() { piExtensionsDir = origDir }()

	proto := PiProtocol{}
	if err := proto.Install("/usr/local/bin/cleo", false); err != nil {
		t.Fatalf("Install: %v", err)
	}

	got, err := os.ReadFile(filepath.Join(extDir, "cleo.ts"))
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(got) != piExtensionTemplate {
		t.Errorf("installed content does not match template\ngot:\n%s\nwant:\n%s", got, piExtensionTemplate)
	}
}

func TestPiProtocol_Install_Force(t *testing.T) {
	dir := t.TempDir()
	extDir := filepath.Join(dir, ".pi", "agent", "extensions")
	origDir := piExtensionsDir
	piExtensionsDir = extDir
	defer func() { piExtensionsDir = origDir }()

	proto := PiProtocol{}
	// First install.
	if err := proto.Install("/usr/local/bin/cleo", false); err != nil {
		t.Fatal(err)
	}
	// Conflict: file already exists and matches — no error even without --force.
	if err := proto.Install("/usr/local/bin/cleo", false); err != nil {
		t.Errorf("re-install with same content should not fail: %v", err)
	}
}

func TestPiProtocol_Cleanup_RemovesMatchingFile(t *testing.T) {
	dir := t.TempDir()
	extDir := filepath.Join(dir, ".pi", "agent", "extensions")
	origDir := piExtensionsDir
	piExtensionsDir = extDir
	defer func() { piExtensionsDir = origDir }()

	proto := PiProtocol{}
	_ = proto.Install("/usr/local/bin/cleo", false)

	if err := proto.Cleanup(); err != nil {
		t.Fatalf("Cleanup: %v", err)
	}
	if _, err := os.Stat(filepath.Join(extDir, "cleo.ts")); !os.IsNotExist(err) {
		t.Error("expected cleo.ts to be removed after Cleanup")
	}
}

func TestExpectedPiEntry_MatchesTemplate(t *testing.T) {
	if ExpectedPiEntry() != piExtensionTemplate {
		t.Error("ExpectedPiEntry() must return the embedded template")
	}
}
```

- [ ] **Step 2: Run tests — expect failure**

```bash
go test ./internal/hooks/... -run "TestPiProtocol_Install|TestPiProtocol_Cleanup|TestExpectedPiEntry" -v
```
Expected: FAIL — `piExtensionsDir`, `piExtensionTemplate`, `ExpectedPiEntry` not defined.

- [ ] **Step 3: Implement `Install()`, `Cleanup()`, `ExpectedPiEntry()` in `pi.go`**

Replace the stub `Install` and `Cleanup` methods and add the template constant:

```go
package hooks

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/dhruvsaxena1998/cleo/internal/state"
)

// piExtensionsDir is the directory where cleo writes its Pi extension.
// Overridden in tests to avoid touching the real home directory.
var piExtensionsDir = ""

func piExtDir() string {
	if piExtensionsDir != "" {
		return piExtensionsDir
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".pi", "agent", "extensions")
}

// piExtensionTemplate is embedded as a constant so cleo doctor can diff the
// on-disk file against it without reading an external file.
const piExtensionTemplate = `// Generated by ` + "`cleo init`" + ` — do not edit manually. Re-run to update.
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
`

// ExpectedPiEntry returns the expected content of ~/.pi/agent/extensions/cleo.ts.
// Used by cleo doctor to diff the on-disk file.
func ExpectedPiEntry() string { return piExtensionTemplate }

func (PiProtocol) Install(cleoBin string, force bool) error {
	dir := piExtDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	dest := filepath.Join(dir, "cleo.ts")
	existing, err := os.ReadFile(dest)
	if err == nil {
		// File exists. If content matches, no-op (idempotent).
		if string(existing) == piExtensionTemplate {
			return nil
		}
		if !force {
			return fmt.Errorf("conflict: %s already exists with different content (re-run with --force to overwrite)", dest)
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return os.WriteFile(dest, []byte(piExtensionTemplate), 0o644)
}

func (PiProtocol) Cleanup() error {
	dest := filepath.Join(piExtDir(), "cleo.ts")
	content, err := os.ReadFile(dest)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if string(content) != piExtensionTemplate {
		fmt.Fprintf(os.Stderr, "warning: %s has been modified; skipping removal\n", dest)
		return nil
	}
	return os.Remove(dest)
}

// Keep Normalize and the rest of the file unchanged from Task 5.
```

- [ ] **Step 4: Run tests — expect pass**

```bash
go test ./internal/hooks/... -run "TestPiProtocol_Install|TestPiProtocol_Cleanup|TestExpectedPiEntry" -v
```
Expected: PASS.

- [ ] **Step 5: Run full test suite**

```bash
go test ./...
```
Expected: all pass.

- [ ] **Step 6: Commit**

```bash
git add internal/hooks/pi.go internal/hooks/pi_test.go
git commit -m "feat(hooks): implement PiProtocol.Install/Cleanup and TypeScript extension template"
```

---

## Task 7: Add Pi to `cleo init`

**Files:**
- Modify: `internal/cli/init.go`

- [ ] **Step 1: Update `init.go`**

Add `hookPi` constant and Pi case. Also update `InstallClaude`/`InstallCodex` call sites to go through `Protocol.Install()` for consistency.

In `internal/cli/init.go`:

```go
const (
	hookClaude = "claude"
	hookCodex  = "codex"
	hookPi     = "pi"
)
```

Update the multi-select options in `promptHookSelection`:

```go
func promptHookSelection(selected *[]string) error {
	return huh.NewForm(
		huh.NewGroup(
			huh.NewMultiSelect[string]().
				Title("Which hook systems would you like to install?").
				Options(
					huh.NewOption("Claude Code  (~/.claude/settings.json)", hookClaude),
					huh.NewOption("Codex        (~/.codex/hooks.json)", hookCodex),
					huh.NewOption("Pi           (~/.pi/agent/extensions/cleo.ts)", hookPi),
				).
				Value(selected),
		),
	).Run()
}
```

Update the switch in `RunE` to add the Pi case and use `Protocol.Install()`:

```go
for _, h := range selected {
	switch h {
	case hookClaude:
		if err := hooks.ClaudeProtocol{}.Install(cleoBin, force); err != nil {
			return err
		}
		results = append(results, initInstallResult{
			Name: "Claude Code",
			Files: []string{
				fmt.Sprintf("hooks: %s", filepath.Join(home, ".claude", "settings.json")),
			},
			InstalledHooks: hooks.ClaudeEvents(),
		})
	case hookCodex:
		if err := hooks.CodexProtocol{}.Install(cleoBin, force); err != nil {
			return err
		}
		results = append(results, initInstallResult{
			Name: "Codex",
			Files: []string{
				fmt.Sprintf("hooks:        %s", filepath.Join(home, ".codex", "hooks.json")),
				fmt.Sprintf("feature flag: %s ([features].hooks = true)", filepath.Join(home, ".codex", "config.toml")),
			},
			InstalledHooks:   hooks.CodexEvents(),
			NeedsCodexReview: true,
			ReviewHooks:      hooks.CodexEvents(),
			ReviewCommand:    fmt.Sprintf("%s hook codex", cleoBin),
		})
	case hookPi:
		if err := hooks.PiProtocol{}.Install(cleoBin, force); err != nil {
			return err
		}
		results = append(results, initInstallResult{
			Name: "Pi",
			Files: []string{
				fmt.Sprintf("extension: %s", filepath.Join(home, ".pi", "agent", "extensions", "cleo.ts")),
			},
			InstalledHooks: hooks.PiEvents(),
		})
	}
}
```

Also update `selected` default (Pi is opt-in, not selected by default):

```go
selected := []string{hookClaude, hookCodex} // Pi is opt-in
```

- [ ] **Step 2: Build and verify compilation**

```bash
go build ./...
```
Expected: builds without errors.

- [ ] **Step 3: Run tests**

```bash
go test ./internal/cli/... -v
```
Expected: all pass.

- [ ] **Step 4: Commit**

```bash
git add internal/cli/init.go
git commit -m "feat(init): add Pi to hook installation multi-select"
```

---

## Task 8: Reformat `printInitSummary` with lipgloss

**Files:**
- Modify: `internal/cli/init.go`
- Create: `internal/cli/init_test.go`

- [ ] **Step 1: Write the snapshot test**

Create `internal/cli/init_test.go`:

```go
package cli

import (
	"bytes"
	"strings"
	"testing"
)

func TestPrintInitSummary_Claude(t *testing.T) {
	var buf bytes.Buffer
	printInitSummary(&buf, []initInstallResult{
		{
			Name:           "Claude Code",
			Files:          []string{"hooks: /home/user/.claude/settings.json"},
			InstalledHooks: []string{"SessionStart", "UserPromptSubmit", "PreToolUse"},
		},
	})
	out := stripANSI(buf.String())

	wantStrings := []string{
		"Cleo hooks initialized",
		"Claude Code",
		"hooks",
		"/home/user/.claude/settings.json",
		"SessionStart",
		"UserPromptSubmit",
		"PreToolUse",
	}
	for _, want := range wantStrings {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q\ngot:\n%s", want, out)
		}
	}
}

func TestPrintInitSummary_CodexApprovalBlock(t *testing.T) {
	var buf bytes.Buffer
	printInitSummary(&buf, []initInstallResult{
		{
			Name:             "Codex",
			Files:            []string{"hooks: /home/user/.codex/hooks.json"},
			InstalledHooks:   []string{"SessionStart", "Stop"},
			NeedsCodexReview: true,
			ReviewHooks:      []string{"SessionStart", "Stop"},
			ReviewCommand:    "/usr/local/bin/cleo hook codex",
		},
	})
	out := stripANSI(buf.String())

	if !strings.Contains(out, "manual hook approval") {
		t.Errorf("output missing approval warning\ngot:\n%s", out)
	}
	if !strings.Contains(out, "/hooks") {
		t.Errorf("output missing /hooks instruction\ngot:\n%s", out)
	}
	if !strings.Contains(out, "/usr/local/bin/cleo hook codex") {
		t.Errorf("output missing review command\ngot:\n%s", out)
	}
}

// stripANSI removes ANSI escape sequences for plain-text assertions.
func stripANSI(s string) string {
	var out strings.Builder
	inEsc := false
	for _, r := range s {
		if r == '\x1b' {
			inEsc = true
			continue
		}
		if inEsc {
			if r == 'm' {
				inEsc = false
			}
			continue
		}
		out.WriteRune(r)
	}
	return out.String()
}
```

- [ ] **Step 2: Run test — expect pass on current output (baseline)**

```bash
go test ./internal/cli/... -run TestPrintInitSummary -v
```
The tests assert on content, not exact formatting, so they should pass even before the lipgloss rewrite. If they fail, fix the assertions to match the actual current output.

- [ ] **Step 3: Rewrite `printInitSummary` in `init.go`**

Replace the current `printInitSummary` function:

```go
import (
	// add to existing imports:
	"strings"
	"github.com/charmbracelet/lipgloss"
)

var (
	initHeaderStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#a6e3a1"))
	initAgentStyle  = lipgloss.NewStyle().Bold(true)
	initOkStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("#a6e3a1"))
	initWarnStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("#f38ba8"))
	initDimStyle    = lipgloss.NewStyle().Faint(true)
	initLabelWidth  = 10
)

func printInitSummary(w io.Writer, results []initInstallResult) {
	fmt.Fprintln(w, initHeaderStyle.Render("✓ Cleo hooks initialized"))
	fmt.Fprintln(w)

	for _, result := range results {
		fmt.Fprintf(w, "  %s\n", initAgentStyle.Render(result.Name))
		for _, file := range result.Files {
			// Split "label: path" into label + path for styled output.
			label, path, found := strings.Cut(file, ": ")
			if !found {
				fmt.Fprintf(w, "  %s %s\n", initOkStyle.Render("✓"), file)
				continue
			}
			fmt.Fprintf(w, "  %s %-*s %s\n",
				initOkStyle.Render("✓"),
				initLabelWidth, label,
				initDimStyle.Render(path),
			)
		}
		if len(result.InstalledHooks) > 0 {
			// Wrap event names into rows of 4.
			const perRow = 4
			for i := 0; i < len(result.InstalledHooks); i += perRow {
				end := i + perRow
				if end > len(result.InstalledHooks) {
					end = len(result.InstalledHooks)
				}
				chunk := strings.Join(result.InstalledHooks[i:end], " · ")
				if i == 0 {
					fmt.Fprintf(w, "  %s %-*s %s\n",
						initOkStyle.Render("✓"),
						initLabelWidth, fmt.Sprintf("%d events", len(result.InstalledHooks)),
						chunk,
					)
				} else {
					fmt.Fprintf(w, "  %s %-*s %s\n", " ", initLabelWidth+2, "", chunk)
				}
			}
		}
		fmt.Fprintln(w)
	}

	for _, result := range results {
		if !result.NeedsCodexReview {
			continue
		}
		fmt.Fprintf(w, "%s  %s requires manual hook approval\n",
			initWarnStyle.Render("⚠"), result.Name)
		fmt.Fprintf(w, "   Open %s, run /hooks, and approve entries starting with:\n", result.Name)
		fmt.Fprintf(w, "   %s\n", initDimStyle.Render(result.ReviewCommand))
		fmt.Fprintln(w, "   Restart any open sessions first so they pick up the updated config.")
		fmt.Fprintln(w)
	}
}
```

- [ ] **Step 4: Run tests — expect pass**

```bash
go test ./internal/cli/... -run TestPrintInitSummary -v
```
Expected: PASS.

- [ ] **Step 5: Run full test suite**

```bash
go test ./...
```
Expected: all pass.

- [ ] **Step 6: Commit**

```bash
git add internal/cli/init.go internal/cli/init_test.go
git commit -m "feat(init): reformat output with lipgloss colors and structured layout"
```

---

## Task 9: Add Pi check to `cleo doctor`

**Files:**
- Modify: `internal/cli/doctor.go`
- Modify: `internal/cli/doctor_test.go`

- [ ] **Step 1: Read how Claude/Codex checks work in `doctor.go`**

Find the section where Claude hook installation is checked (search for `ExpectedClaudeEntries` or `checkClaude`). The Pi check follows the same pattern but checks a file path instead of JSON entries.

- [ ] **Step 2: Write the failing tests**

Add to `internal/cli/doctor_test.go`:

```go
func TestDoctorPiCheck_FileMissing(t *testing.T) {
	dir := t.TempDir()
	// No Pi extension file exists.
	report := doctorReport{
		PiExtensionPath: filepath.Join(dir, ".pi", "agent", "extensions", "cleo.ts"),
		CleoBin:         "/usr/local/bin/cleo",
	}
	result := checkPi(report)
	if result.ok {
		t.Error("expected not-ok when extension file is missing")
	}
	if !strings.Contains(result.message, "not found") && !strings.Contains(result.message, "missing") {
		t.Errorf("expected missing/not-found in message, got: %q", result.message)
	}
}

func TestDoctorPiCheck_FileMatches(t *testing.T) {
	dir := t.TempDir()
	extDir := filepath.Join(dir, ".pi", "agent", "extensions")
	_ = os.MkdirAll(extDir, 0o755)
	dest := filepath.Join(extDir, "cleo.ts")
	_ = os.WriteFile(dest, []byte(hooks.ExpectedPiEntry()), 0o644)

	report := doctorReport{
		PiExtensionPath: dest,
		CleoBin:         "/usr/local/bin/cleo",
	}
	result := checkPi(report)
	if !result.ok {
		t.Errorf("expected ok when extension matches template, got: %q", result.message)
	}
}

func TestDoctorPiCheck_FileStale(t *testing.T) {
	dir := t.TempDir()
	extDir := filepath.Join(dir, ".pi", "agent", "extensions")
	_ = os.MkdirAll(extDir, 0o755)
	dest := filepath.Join(extDir, "cleo.ts")
	_ = os.WriteFile(dest, []byte("// old content"), 0o644)

	report := doctorReport{
		PiExtensionPath: dest,
		CleoBin:         "/usr/local/bin/cleo",
	}
	result := checkPi(report)
	if result.ok {
		t.Error("expected not-ok when extension is stale")
	}
	if !strings.Contains(result.message, "stale") && !strings.Contains(result.message, "re-run") {
		t.Errorf("expected stale/re-run in message, got: %q", result.message)
	}
}
```

Also add `PiExtensionPath` to the `doctorReport` struct if it doesn't already have it.

- [ ] **Step 3: Run tests — expect failure**

```bash
go test ./internal/cli/... -run TestDoctorPiCheck -v
```
Expected: FAIL — `checkPi`, `PiExtensionPath` not defined.

- [ ] **Step 4: Implement the Pi check in `doctor.go`**

Add `PiExtensionPath string` to the `doctorReport` struct.

Populate it in `newDoctorReport()` (or wherever `doctorReport` is built):
```go
home, _ := os.UserHomeDir()
report.PiExtensionPath = filepath.Join(home, ".pi", "agent", "extensions", "cleo.ts")
```

Add the `checkPi` function and a `checkResult` helper type (use whatever the existing doctor check functions return):

```go
func checkPi(report doctorReport) checkResult {
	content, err := os.ReadFile(report.PiExtensionPath)
	if os.IsNotExist(err) {
		return checkResult{
			ok:      false,
			message: fmt.Sprintf("Pi extension not found: %s — run cleo init to install", report.PiExtensionPath),
		}
	}
	if err != nil {
		return checkResult{ok: false, message: fmt.Sprintf("Pi extension: %v", err)}
	}
	if string(content) != hooks.ExpectedPiEntry() {
		return checkResult{
			ok:      false,
			message: fmt.Sprintf("Pi extension is stale: re-run `cleo init` to update %s", report.PiExtensionPath),
		}
	}
	return checkResult{ok: true, message: fmt.Sprintf("Pi extension: %s", report.PiExtensionPath)}
}
```

Wire `checkPi` into the doctor output section alongside the Claude and Codex checks.

- [ ] **Step 5: Run tests — expect pass**

```bash
go test ./internal/cli/... -run TestDoctorPiCheck -v
```
Expected: PASS.

- [ ] **Step 6: Run full test suite**

```bash
go test ./...
```
Expected: all pass.

- [ ] **Step 7: Commit**

```bash
git add internal/cli/doctor.go internal/cli/doctor_test.go
git commit -m "feat(doctor): add Pi extension check (file exists + content diff)"
```

---

## Final verification

- [ ] **Run the full test suite one last time**

```bash
go test ./... -count=1
```
Expected: all pass, no cached results.

- [ ] **Build the binary**

```bash
make build && ./bin/cleo --version
```
Expected: builds and prints version.

- [ ] **Manual smoke**

```bash
./bin/cleo init --help       # Pi should appear in help text
./bin/cleo doctor            # Pi check should appear
./bin/cleo hook pi session_start --payload '{"cwd":"/tmp"}'  # Should not error
```
