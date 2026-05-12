# OpenCode Hook Integration — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a full opencode hook protocol to cleo so opencode sessions have TUI state tracking, sound events, and doctor coverage — matching the existing claude/codex/pi integrations.

**Architecture:** A new `OpenCodeProtocol` in `internal/hooks/opencode.go` ships a generated TypeScript plugin to `~/.config/opencode/plugins/cleo.ts`. The plugin calls `cleo hook opencode <event> --payload <json>` for 7 lifecycle events. `cleo init` installs it; `cleo doctor` checks it.

**Tech Stack:** Go, existing cleo `Protocol` interface, Bun TypeScript plugin API (opencode)

**Spec:** `docs/superpowers/specs/2026-05-12-opencode-hook-integration-design.md`

---

### Task 1: Implement `OpenCodeProtocol.Normalize` with tests

**Files:**
- Create: `internal/hooks/opencode.go`
- Create: `internal/hooks/opencode_test.go`

- [ ] **Step 1.1: Write failing tests for Normalize and Metadata**

Create `internal/hooks/opencode_test.go`:

```go
package hooks

import (
	"testing"

	"github.com/dhruvsaxena1998/cleo/internal/state"
)

func TestOpenCodeProtocol_Normalize(t *testing.T) {
	proto := OpenCodeProtocol{}

	tests := []struct {
		event   string
		payload string
		want    NormalizedEvent
		wantOk  bool
	}{
		{
			event:   "session.created",
			payload: `{"cwd":"/proj","session_id":"cleo-x-opencode-1"}`,
			want:    NormalizedEvent{StateEvent: state.EvSessionStart, SoundEvent: "session_start"},
			wantOk:  true,
		},
		{
			event:   "tool.execute.before",
			payload: `{"cwd":"/proj","session_id":"cleo-x-opencode-1","tool_name":"write"}`,
			want:    NormalizedEvent{StateEvent: state.EvPreToolUse, ToolName: "write"},
			wantOk:  true,
		},
		{
			event:   "tool.execute.after",
			payload: `{"cwd":"/proj","session_id":"cleo-x-opencode-1","tool_name":"write"}`,
			want:    NormalizedEvent{StateEvent: state.EvPostToolUse, ToolName: "write"},
			wantOk:  true,
		},
		{
			event:   "permission.asked",
			payload: `{"cwd":"/proj","session_id":"cleo-x-opencode-1"}`,
			want:    NormalizedEvent{StateEvent: state.EvNotification, SoundEvent: "needs_input", SuppressWhenIdle: false},
			wantOk:  true,
		},
		{
			event:   "session.idle",
			payload: `{"cwd":"/proj","session_id":"cleo-x-opencode-1"}`,
			want:    NormalizedEvent{StateEvent: state.EvStop, SoundEvent: "session_idle"},
			wantOk:  true,
		},
		{
			event:   "session.deleted",
			payload: `{"cwd":"/proj","session_id":"cleo-x-opencode-1"}`,
			want:    NormalizedEvent{StateEvent: state.EvSessionEnd, SoundEvent: "session_completed"},
			wantOk:  true,
		},
		{
			event:   "session.error",
			payload: `{"cwd":"/proj","session_id":"cleo-x-opencode-1"}`,
			want:    NormalizedEvent{StateEvent: state.EvError, SoundEvent: "session_error"},
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

func TestOpenCodeProtocol_Metadata(t *testing.T) {
	proto := OpenCodeProtocol{}
	if proto.Name() != "opencode" {
		t.Errorf("Name() = %q, want \"opencode\"", proto.Name())
	}
	if !proto.UsesCwdFallback() {
		t.Error("OpenCode must use cwd fallback")
	}
	if len(proto.Events()) == 0 {
		t.Error("Events() returned empty slice")
	}
	events := proto.Events()
	wantEvents := []string{
		"session.created", "tool.execute.before", "tool.execute.after",
		"permission.asked", "session.idle", "session.deleted", "session.error",
	}
	if len(events) != len(wantEvents) {
		t.Fatalf("Events() len = %d, want %d", len(events), len(wantEvents))
	}
	for i, want := range wantEvents {
		if events[i] != want {
			t.Errorf("Events()[%d] = %q, want %q", i, events[i], want)
		}
	}
}
```

- [ ] **Step 1.2: Run tests to verify they fail (file not yet created)**

```
cd /path/to/cleo && go test ./internal/hooks/ -run TestOpenCode -v
```

Expected: compile error — `OpenCodeProtocol undefined`

- [ ] **Step 1.3: Create `internal/hooks/opencode.go` with Normalize and Metadata**

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

var opencodeEvents = []string{
	"session.created", "tool.execute.before", "tool.execute.after",
	"permission.asked", "session.idle", "session.deleted", "session.error",
}

// OpenCodeEvents returns the lifecycle event names cleo subscribes to in opencode.
func OpenCodeEvents() []string { return append([]string(nil), opencodeEvents...) }

// openCodePluginsDir is the directory where cleo writes its opencode plugin.
// Overridden in tests to avoid touching the real home directory.
var openCodePluginsDir = ""

func openCodePlugDir() string {
	if openCodePluginsDir != "" {
		return openCodePluginsDir
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "opencode", "plugins")
}

// openCodePluginTemplate is the canonical content of
// ~/.config/opencode/plugins/cleo.ts. Stored as a constant so cleo doctor
// can diff on-disk content without reading an external file.
const openCodePluginTemplate = `// Generated by ` + "`cleo init`" + ` — do not edit manually. Re-run to update.
export default async function({ $, directory }) {
  const hook = async (event, extra = {}) => {
    const payload = JSON.stringify({ cwd: directory, ...extra });
    await $` + "`" + `cleo hook opencode ${event} --payload ${payload}` + "`" + `;
  };

  return {
    hooks: {
      "session.created":     async ({ sessionID }) => hook("session.created",     { session_id: sessionID }),
      "tool.execute.before": async ({ sessionID, tool }) => hook("tool.execute.before", { session_id: sessionID, tool_name: tool }),
      "tool.execute.after":  async ({ sessionID, tool }) => hook("tool.execute.after",  { session_id: sessionID, tool_name: tool }),
      "permission.asked":    async ({ sessionID }) => hook("permission.asked",    { session_id: sessionID }),
      "session.idle":        async ({ sessionID }) => hook("session.idle",        { session_id: sessionID }),
      "session.deleted":     async ({ sessionID }) => hook("session.deleted",     { session_id: sessionID }),
      "session.error":       async ({ sessionID }) => hook("session.error",       { session_id: sessionID }),
    },
  };
}
`

// ExpectedOpenCodeEntry returns the expected content of
// ~/.config/opencode/plugins/cleo.ts. Used by cleo doctor to diff the on-disk file.
func ExpectedOpenCodeEntry() string { return openCodePluginTemplate }

type OpenCodeProtocol struct{}

func (OpenCodeProtocol) Name() string          { return "opencode" }
func (OpenCodeProtocol) Events() []string      { return OpenCodeEvents() }
func (OpenCodeProtocol) UsesCwdFallback() bool { return true }

func (OpenCodeProtocol) Install(cleoBin string, force bool) error {
	dir := openCodePlugDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	dest := filepath.Join(dir, "cleo.ts")
	existing, err := os.ReadFile(dest)
	if err == nil {
		if string(existing) == openCodePluginTemplate {
			return nil // already up-to-date; idempotent
		}
		if !force {
			return fmt.Errorf("conflict: %s already exists with different content (re-run with --force to overwrite)", dest)
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return os.WriteFile(dest, []byte(openCodePluginTemplate), 0o644)
}

func (OpenCodeProtocol) Cleanup() error {
	dest := filepath.Join(openCodePlugDir(), "cleo.ts")
	content, err := os.ReadFile(dest)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if string(content) != openCodePluginTemplate {
		fmt.Fprintf(os.Stderr, "warning: %s has been modified; skipping removal\n", dest)
		return nil
	}
	return os.Remove(dest)
}

func (OpenCodeProtocol) Normalize(event string, payload []byte) (NormalizedEvent, bool) {
	var p struct {
		ToolName string `json:"tool_name"`
	}
	_ = json.Unmarshal(payload, &p)

	switch event {
	case "session.created":
		return NormalizedEvent{StateEvent: state.EvSessionStart, SoundEvent: "session_start"}, true
	case "tool.execute.before":
		return NormalizedEvent{StateEvent: state.EvPreToolUse, ToolName: p.ToolName}, true
	case "tool.execute.after":
		return NormalizedEvent{StateEvent: state.EvPostToolUse, ToolName: p.ToolName}, true
	case "permission.asked":
		return NormalizedEvent{StateEvent: state.EvNotification, SoundEvent: "needs_input", SuppressWhenIdle: false}, true
	case "session.idle":
		return NormalizedEvent{StateEvent: state.EvStop, SoundEvent: "session_idle"}, true
	case "session.deleted":
		return NormalizedEvent{StateEvent: state.EvSessionEnd, SoundEvent: "session_completed"}, true
	case "session.error":
		return NormalizedEvent{StateEvent: state.EvError, SoundEvent: "session_error"}, true
	}
	return NormalizedEvent{}, false
}
```

- [ ] **Step 1.4: Run tests to verify they pass**

```
go test ./internal/hooks/ -run TestOpenCode -v
```

Expected: all `TestOpenCodeProtocol_Normalize` subtests and `TestOpenCodeProtocol_Metadata` PASS

- [ ] **Step 1.5: Commit**

```bash
git add internal/hooks/opencode.go internal/hooks/opencode_test.go
git commit -m "feat(hooks): add OpenCodeProtocol with Normalize and event map"
```

---

### Task 2: Implement Install, Cleanup, and register the protocol

**Files:**
- Modify: `internal/hooks/opencode_test.go` (add Install/Cleanup tests)
- Modify: `internal/hooks/protocol.go` (register OpenCodeProtocol)

- [ ] **Step 2.1: Add Install and Cleanup tests to `internal/hooks/opencode_test.go`**

Append these test functions to the existing file:

```go
func TestOpenCodeProtocol_Install_WritesPlugin(t *testing.T) {
	dir := t.TempDir()
	plugDir := filepath.Join(dir, "plugins")

	origDir := openCodePluginsDir
	openCodePluginsDir = plugDir
	defer func() { openCodePluginsDir = origDir }()

	proto := OpenCodeProtocol{}
	if err := proto.Install("/usr/local/bin/cleo", false); err != nil {
		t.Fatalf("Install: %v", err)
	}

	got, err := os.ReadFile(filepath.Join(plugDir, "cleo.ts"))
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(got) != openCodePluginTemplate {
		t.Errorf("installed content does not match template\ngot:\n%s\nwant:\n%s", got, openCodePluginTemplate)
	}
}

func TestOpenCodeProtocol_Install_Idempotent(t *testing.T) {
	dir := t.TempDir()
	plugDir := filepath.Join(dir, "plugins")
	origDir := openCodePluginsDir
	openCodePluginsDir = plugDir
	defer func() { openCodePluginsDir = origDir }()

	proto := OpenCodeProtocol{}
	if err := proto.Install("/usr/local/bin/cleo", false); err != nil {
		t.Fatal(err)
	}
	if err := proto.Install("/usr/local/bin/cleo", false); err != nil {
		t.Errorf("re-install with same content should not fail: %v", err)
	}
}

func TestOpenCodeProtocol_Install_ConflictWithoutForce(t *testing.T) {
	dir := t.TempDir()
	plugDir := filepath.Join(dir, "plugins")
	origDir := openCodePluginsDir
	openCodePluginsDir = plugDir
	defer func() { openCodePluginsDir = origDir }()

	_ = os.MkdirAll(plugDir, 0o755)
	_ = os.WriteFile(filepath.Join(plugDir, "cleo.ts"), []byte("// different content"), 0o644)

	proto := OpenCodeProtocol{}
	if err := proto.Install("/usr/local/bin/cleo", false); err == nil {
		t.Error("expected conflict error, got nil")
	}
}

func TestOpenCodeProtocol_Install_ForceOverwrites(t *testing.T) {
	dir := t.TempDir()
	plugDir := filepath.Join(dir, "plugins")
	origDir := openCodePluginsDir
	openCodePluginsDir = plugDir
	defer func() { openCodePluginsDir = origDir }()

	_ = os.MkdirAll(plugDir, 0o755)
	_ = os.WriteFile(filepath.Join(plugDir, "cleo.ts"), []byte("// different content"), 0o644)

	proto := OpenCodeProtocol{}
	if err := proto.Install("/usr/local/bin/cleo", true); err != nil {
		t.Fatalf("Install with --force: %v", err)
	}
	got, _ := os.ReadFile(filepath.Join(plugDir, "cleo.ts"))
	if string(got) != openCodePluginTemplate {
		t.Error("force install did not overwrite with template")
	}
}

func TestOpenCodeProtocol_Cleanup_RemovesMatchingFile(t *testing.T) {
	dir := t.TempDir()
	plugDir := filepath.Join(dir, "plugins")
	origDir := openCodePluginsDir
	openCodePluginsDir = plugDir
	defer func() { openCodePluginsDir = origDir }()

	proto := OpenCodeProtocol{}
	_ = proto.Install("/usr/local/bin/cleo", false)

	if err := proto.Cleanup(); err != nil {
		t.Fatalf("Cleanup: %v", err)
	}
	if _, err := os.Stat(filepath.Join(plugDir, "cleo.ts")); !os.IsNotExist(err) {
		t.Error("expected cleo.ts to be removed after Cleanup")
	}
}

func TestOpenCodeProtocol_Cleanup_SkipsModifiedFile(t *testing.T) {
	dir := t.TempDir()
	plugDir := filepath.Join(dir, "plugins")
	origDir := openCodePluginsDir
	openCodePluginsDir = plugDir
	defer func() { openCodePluginsDir = origDir }()

	_ = os.MkdirAll(plugDir, 0o755)
	dest := filepath.Join(plugDir, "cleo.ts")
	_ = os.WriteFile(dest, []byte("// user-modified"), 0o644)

	proto := OpenCodeProtocol{}
	if err := proto.Cleanup(); err != nil {
		t.Fatalf("Cleanup returned error for modified file: %v", err)
	}
	if _, err := os.Stat(dest); os.IsNotExist(err) {
		t.Error("Cleanup must NOT remove a user-modified file")
	}
}

func TestExpectedOpenCodeEntry_MatchesTemplate(t *testing.T) {
	if ExpectedOpenCodeEntry() != openCodePluginTemplate {
		t.Error("ExpectedOpenCodeEntry() must return the embedded template")
	}
}

func TestResolveSession_CwdFallbackCalledForOpenCode(t *testing.T) {
	deps, st, _ := setup(t)
	_ = st.Put(state.Session{ID: "cleo-x-opencode-1", Agent: "opencode", State: state.Running})
	deps.Now = func() (string, error) { return "", fmt.Errorf("not set") }
	called := false
	deps.FindByCwd = func(cwd, agent string) (string, error) {
		called = true
		if agent == "opencode" {
			return "cleo-x-opencode-1", nil
		}
		return "", nil
	}
	_ = Handle(deps, "opencode", "session.created", []byte(`{"cwd":"/proj"}`))
	if !called {
		t.Error("FindByCwd must be called for OpenCodeProtocol (UsesCwdFallback=true) when env absent")
	}
}
```

Also add `"fmt"` and `"os"` and `"path/filepath"` imports if not present. The file already imports `"testing"` and `"github.com/dhruvsaxena1998/cleo/internal/state"` — add the missing ones:

```go
import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/dhruvsaxena1998/cleo/internal/state"
)
```

- [ ] **Step 2.2: Run tests to verify new tests fail**

```
go test ./internal/hooks/ -run "TestOpenCodeProtocol_Install|TestOpenCodeProtocol_Cleanup|TestExpectedOpenCode|TestResolveSession_CwdFallbackCalledForOpenCode" -v
```

Expected: FAIL — `openCodePluginsDir undefined` (because opencode.go doesn't have the var/Install/Cleanup yet — but we wrote them in Task 1, so they exist). If Task 1 is done, these should PASS already. Move on.

- [ ] **Step 2.3: Register `OpenCodeProtocol{}` in `internal/hooks/protocol.go`**

In `internal/hooks/protocol.go`, find the `Protocols()` function:

```go
func Protocols() []Protocol {
	return []Protocol{
		ClaudeProtocol{},
		CodexProtocol{},
		PiProtocol{},
	}
}
```

Change it to:

```go
func Protocols() []Protocol {
	return []Protocol{
		ClaudeProtocol{},
		CodexProtocol{},
		PiProtocol{},
		OpenCodeProtocol{},
	}
}
```

Also add `type OpenCodeProtocol struct{}` to the type declarations block at the bottom of the file. Find:

```go
// Protocol type declarations. Methods are implemented in each protocol's own
// file (claude.go, codex.go, pi.go).
type ClaudeProtocol struct{}
type CodexProtocol struct{}
```

Change it to:

```go
// Protocol type declarations. Methods are implemented in each protocol's own
// file (claude.go, codex.go, pi.go, opencode.go).
type ClaudeProtocol struct{}
type CodexProtocol struct{}
type OpenCodeProtocol struct{}
```

- [ ] **Step 2.4: Run all hooks package tests**

```
go test ./internal/hooks/... -v
```

Expected: all tests PASS

- [ ] **Step 2.5: Commit**

```bash
git add internal/hooks/opencode_test.go internal/hooks/protocol.go
git commit -m "feat(hooks): register OpenCodeProtocol and add Install/Cleanup tests"
```

---

### Task 3: Update defaults and wire `cleo init`

**Files:**
- Modify: `internal/config/defaults.go` (1 line)
- Modify: `internal/cli/init.go`
- Modify: `internal/cli/init_test.go`

- [ ] **Step 3.1: Update opencode agent's `Hooks` field in `internal/config/defaults.go`**

Find:

```go
"opencode": {Command: "opencode", Label: "oc", Color: "#FF6B35", Hooks: "none"},
```

Change to:

```go
"opencode": {Command: "opencode", Label: "oc", Color: "#FF6B35", Hooks: "opencode"},
```

- [ ] **Step 3.2: Add opencode to `internal/cli/init.go`**

**Step 3.2a** — Add `hookOpenCode` constant. Find the const block:

```go
const (
	hookClaude = "claude"
	hookCodex  = "codex"
	hookPi     = "pi"
)
```

Change to:

```go
const (
	hookClaude    = "claude"
	hookCodex     = "codex"
	hookPi        = "pi"
	hookOpenCode  = "opencode"
)
```

**Step 3.2b** — Add `case hookOpenCode` to the install switch. Find the Pi case (last case in the switch):

```go
			case hookPi:
				if err := (hooks.PiProtocol{}).Install(cleoBin, force); err != nil {
					return err
				}
				results = append(results, initInstallResult{
					Name: "Pi",
					Files: []string{
						fmt.Sprintf("extension: %s", filepath.Join(home, ".pi", "agent", "extensions", "cleo.ts")),
					},
					InstalledHooks: hooks.PiEvents(),
				})
```

Insert the opencode case directly after the Pi case (before the closing `}` of the switch):

```go
			case hookOpenCode:
				if err := (hooks.OpenCodeProtocol{}).Install(cleoBin, force); err != nil {
					return err
				}
				results = append(results, initInstallResult{
					Name: "OpenCode",
					Files: []string{
						fmt.Sprintf("plugin: %s", filepath.Join(home, ".config", "opencode", "plugins", "cleo.ts")),
					},
					InstalledHooks: hooks.OpenCodeEvents(),
				})
```

**Step 3.2c** — Add opencode to `promptHookSelection`. Find the `opts` slice:

```go
	opts := []hookOpt{
		{hookClaude, "Claude Code  (~/.claude/settings.json)", true},
		{hookCodex, "Codex        (~/.codex/hooks.json)", true},
		{hookPi, "Pi           (~/.pi/agent/extensions/cleo.ts)", false},
	}
```

Change to:

```go
	opts := []hookOpt{
		{hookClaude, "Claude Code  (~/.claude/settings.json)", true},
		{hookCodex, "Codex        (~/.codex/hooks.json)", true},
		{hookPi, "Pi           (~/.pi/agent/extensions/cleo.ts)", false},
		{hookOpenCode, "OpenCode     (~/.config/opencode/plugins/cleo.ts)", false},
	}
```

- [ ] **Step 3.3: Update `TestPromptHookSelection` in `internal/cli/init_test.go`**

The test currently has 3 prompts per run. With opencode added as a 4th option (default: no), update all test cases. Find and replace the entire `TestPromptHookSelection` test:

```go
func TestPromptHookSelection(t *testing.T) {
	tests := []struct {
		name     string
		input    string // one line per agent: claude, codex, pi, opencode
		wantKeys []string
	}{
		{
			name:     "all defaults (enter×4)",
			input:    "\n\n\n\n",
			wantKeys: []string{hookClaude, hookCodex}, // pi and opencode default to no
		},
		{
			name:     "select all",
			input:    "y\ny\ny\ny\n",
			wantKeys: []string{hookClaude, hookCodex, hookPi, hookOpenCode},
		},
		{
			name:     "claude only",
			input:    "y\nn\nn\nn\n",
			wantKeys: []string{hookClaude},
		},
		{
			name:     "none selected",
			input:    "n\nn\nn\nn\n",
			wantKeys: []string{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			br := bufio.NewReader(strings.NewReader(tt.input))
			var w bytes.Buffer
			var selected []string
			if err := promptHookSelection(&w, br, &selected); err != nil {
				t.Fatal(err)
			}
			if len(selected) != len(tt.wantKeys) {
				t.Fatalf("got %v, want %v", selected, tt.wantKeys)
			}
			for i, k := range tt.wantKeys {
				if selected[i] != k {
					t.Errorf("index %d: got %q, want %q", i, selected[i], k)
				}
			}
		})
	}
}
```

Also add a print test for the opencode init summary. Append this test to `init_test.go`:

```go
func TestPrintInitSummary_OpenCode(t *testing.T) {
	var buf bytes.Buffer
	printInitSummary(&buf, []initInstallResult{
		{
			Name:           "OpenCode",
			Files:          []string{"plugin: /home/user/.config/opencode/plugins/cleo.ts"},
			InstalledHooks: []string{"session.created", "tool.execute.before", "tool.execute.after", "permission.asked", "session.idle", "session.deleted", "session.error"},
		},
	})
	out := stripANSI(buf.String())

	for _, want := range []string{
		"Cleo hooks initialized",
		"OpenCode",
		"/home/user/.config/opencode/plugins/cleo.ts",
		"7 events",
		"session.created",
		"session.idle",
		"session.deleted",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q\ngot:\n%s", want, out)
		}
	}
	if strings.Contains(out, "manual hook approval") {
		t.Errorf("opencode should not show the Codex approval step:\n%s", out)
	}
}
```

- [ ] **Step 3.4: Run cli package tests**

```
go test ./internal/cli/... -run "TestPromptHookSelection|TestPrintInitSummary" -v
```

Expected: all PASS

- [ ] **Step 3.5: Commit**

```bash
git add internal/config/defaults.go internal/cli/init.go internal/cli/init_test.go
git commit -m "feat(init): add opencode to hook selection and install flow"
```

---

### Task 4: Wire `cleo doctor` to check the opencode plugin

**Files:**
- Modify: `internal/cli/doctor.go`
- Modify: `internal/cli/doctor_test.go`

- [ ] **Step 4.1: Add opencode fields and check function to `internal/cli/doctor.go`**

**Step 4.1a** — Add `OpenCodePluginPath` to `doctorReport`. Find:

```go
type doctorReport struct {
	Checks             []doctorCheck
	HookTracePath      string
	ClaudeSettingsPath string
	CodexHooksPath     string
	PiExtensionPath    string // ← new
	CleoBin            string
}
```

Change to:

```go
type doctorReport struct {
	Checks               []doctorCheck
	HookTracePath        string
	ClaudeSettingsPath   string
	CodexHooksPath       string
	PiExtensionPath      string
	OpenCodePluginPath   string
	CleoBin              string
}
```

**Step 4.1b** — Add `checkOpenCodeExtension` function. Add this immediately after `checkPiExtension`:

```go
func checkOpenCodeExtension(path string) doctorCheck {
	content, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return doctorCheck{
			Label:  "OpenCode plugin",
			Detail: fmt.Sprintf("not found at %s — run cleo init to install", path),
		}
	}
	if err != nil {
		return doctorCheck{Label: "OpenCode plugin", Detail: err.Error()}
	}
	if string(content) != hooks.ExpectedOpenCodeEntry() {
		return doctorCheck{
			Label:  "OpenCode plugin",
			Detail: fmt.Sprintf("stale — re-run cleo init to update %s", path),
		}
	}
	return doctorCheck{Label: "OpenCode plugin", OK: true, Detail: path}
}
```

**Step 4.1c** — Update `diagnoseHooks` signature and body. Find:

```go
func diagnoseHooks(claudeSettingsPath, codexHooksPath, codexConfigPath, hookTracePath, piExtPath string) doctorReport {
	claude := checkClaudeHooks(claudeSettingsPath)
	claude.Protocol = "claude"
	codexFlag := checkCodexFeatureFlag(codexConfigPath)
	codexHooks := checkCodexHooks(codexHooksPath)
	codexHooks.Protocol = "codex"
	pi := checkPiExtension(piExtPath)
	pi.Protocol = "pi"
	claudeAct := checkHookTrace(hookTracePath, "claude")
	claudeAct.Protocol = "claude"
	codexAct := checkHookTrace(hookTracePath, "codex")
	codexAct.Protocol = "codex"
	return doctorReport{
		Checks:             []doctorCheck{claude, codexFlag, codexHooks, pi, claudeAct, codexAct},
		HookTracePath:      hookTracePath,
		ClaudeSettingsPath: claudeSettingsPath,
		CodexHooksPath:     codexHooksPath,
		PiExtensionPath:    piExtPath,
	}
}
```

Change to:

```go
func diagnoseHooks(claudeSettingsPath, codexHooksPath, codexConfigPath, hookTracePath, piExtPath, openCodePlugPath string) doctorReport {
	claude := checkClaudeHooks(claudeSettingsPath)
	claude.Protocol = "claude"
	codexFlag := checkCodexFeatureFlag(codexConfigPath)
	codexHooks := checkCodexHooks(codexHooksPath)
	codexHooks.Protocol = "codex"
	pi := checkPiExtension(piExtPath)
	pi.Protocol = "pi"
	openCode := checkOpenCodeExtension(openCodePlugPath)
	openCode.Protocol = "opencode"
	claudeAct := checkHookTrace(hookTracePath, "claude")
	claudeAct.Protocol = "claude"
	codexAct := checkHookTrace(hookTracePath, "codex")
	codexAct.Protocol = "codex"
	openCodeAct := checkHookTrace(hookTracePath, "opencode")
	openCodeAct.Protocol = "opencode"
	return doctorReport{
		Checks:             []doctorCheck{claude, codexFlag, codexHooks, pi, openCode, claudeAct, codexAct, openCodeAct},
		HookTracePath:      hookTracePath,
		ClaudeSettingsPath: claudeSettingsPath,
		CodexHooksPath:     codexHooksPath,
		PiExtensionPath:    piExtPath,
		OpenCodePluginPath: openCodePlugPath,
	}
}
```

**Step 4.1d** — Update `newDoctorCmd` to pass the opencode plugin path. Find:

```go
		home, _ := os.UserHomeDir()
		piExtPath := filepath.Join(home, ".pi", "agent", "extensions", "cleo.ts")
		report := diagnoseHooks(
			filepath.Join(home, ".claude", "settings.json"),
			filepath.Join(home, ".codex", "hooks.json"),
			filepath.Join(home, ".codex", "config.toml"),
			c.Paths.HookTraceLog(),
			piExtPath,
		)
```

Change to:

```go
		home, _ := os.UserHomeDir()
		piExtPath := filepath.Join(home, ".pi", "agent", "extensions", "cleo.ts")
		openCodePlugPath := filepath.Join(home, ".config", "opencode", "plugins", "cleo.ts")
		report := diagnoseHooks(
			filepath.Join(home, ".claude", "settings.json"),
			filepath.Join(home, ".codex", "hooks.json"),
			filepath.Join(home, ".codex", "config.toml"),
			c.Paths.HookTraceLog(),
			piExtPath,
			openCodePlugPath,
		)
```

- [ ] **Step 4.2: Update `internal/cli/doctor_test.go`**

**Step 4.2a** — Fix all calls to `diagnoseHooks` that now need a 6th argument. Two tests call it:

In `TestDiagnoseHooksReportsHealthySetup`, find:

```go
	report := diagnoseHooks(claudePath, codexHooksPath, codexConfigPath, tracePath, piExtPath)
```

Change to (add opencode plugin setup above and pass path):

```go
	openCodePlugDir := filepath.Join(dir, ".config", "opencode", "plugins")
	if err := os.MkdirAll(openCodePlugDir, 0o755); err != nil {
		t.Fatal(err)
	}
	openCodePlugPath := filepath.Join(openCodePlugDir, "cleo.ts")
	if err := os.WriteFile(openCodePlugPath, []byte(hooks.ExpectedOpenCodeEntry()), 0o644); err != nil {
		t.Fatal(err)
	}

	report := diagnoseHooks(claudePath, codexHooksPath, codexConfigPath, tracePath, piExtPath, openCodePlugPath)
```

Also update the healthy check assertion to include opencode activity:

```go
	got := fmt.Sprint(report.Checks)
	for _, want := range []string{"Claude hook activity", "Codex hook activity", "opencode hook activity"} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected %q in diagnose checks, got %+v", want, report.Checks)
		}
	}
```

Note: the opencode hook activity check will not be OK (no trace yet) in the healthy setup test — that's expected. Only assert the check is present, not that it's OK. The check for `check.OK` loops all checks, so we need to exclude the activity checks or seed a trace. Seed a trace for opencode too. Add to the existing trace write:

```go
	trace := `{"at":"now","protocol":"claude","event":"Stop","env_session":true,"resolved_session":"cleo-x-claude-1","result":"resolved"}` + "\n" +
		`{"at":"now","protocol":"codex","event":"Stop","env_session":true,"resolved_session":"cleo-x-codex-1","result":"resolved"}` + "\n" +
		`{"at":"now","protocol":"opencode","event":"session.idle","env_session":true,"resolved_session":"cleo-x-opencode-1","result":"resolved"}` + "\n"
```

In `TestDiagnoseHooksReportsMissingCodexHook`, find:

```go
	report := diagnoseHooks(claudePath, codexHooksPath, codexConfigPath, tracePath, "")
```

Change to:

```go
	report := diagnoseHooks(claudePath, codexHooksPath, codexConfigPath, tracePath, "", "")
```

**Step 4.2b** — Add opencode doctor check tests. Append these tests to `doctor_test.go`:

```go
func TestDoctorOpenCodeCheck_FileMissing(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".config", "opencode", "plugins", "cleo.ts")

	check := checkOpenCodeExtension(path)
	if check.OK {
		t.Error("expected not-ok when plugin file is missing")
	}
	if !strings.Contains(check.Detail, "run cleo init") {
		t.Errorf("expected 'run cleo init' in detail, got: %q", check.Detail)
	}
}

func TestDoctorOpenCodeCheck_FileMatches(t *testing.T) {
	dir := t.TempDir()
	plugDir := filepath.Join(dir, ".config", "opencode", "plugins")
	_ = os.MkdirAll(plugDir, 0o755)
	dest := filepath.Join(plugDir, "cleo.ts")
	_ = os.WriteFile(dest, []byte(hooks.ExpectedOpenCodeEntry()), 0o644)

	check := checkOpenCodeExtension(dest)
	if !check.OK {
		t.Errorf("expected ok when plugin matches template, got: %q", check.Detail)
	}
}

func TestDoctorOpenCodeCheck_FileStale(t *testing.T) {
	dir := t.TempDir()
	plugDir := filepath.Join(dir, ".config", "opencode", "plugins")
	_ = os.MkdirAll(plugDir, 0o755)
	dest := filepath.Join(plugDir, "cleo.ts")
	_ = os.WriteFile(dest, []byte("// old content"), 0o644)

	check := checkOpenCodeExtension(dest)
	if check.OK {
		t.Error("expected not-ok when plugin is stale")
	}
	if !strings.Contains(check.Detail, "stale") && !strings.Contains(check.Detail, "re-run") {
		t.Errorf("expected 'stale' or 're-run' in detail, got: %q", check.Detail)
	}
}
```

- [ ] **Step 4.3: Run all tests**

```
go test ./internal/... -v 2>&1 | tail -40
```

Expected: all PASS

- [ ] **Step 4.4: Commit**

```bash
git add internal/cli/doctor.go internal/cli/doctor_test.go
git commit -m "feat(doctor): add opencode plugin check and hook activity trace"
```

---

### Task 5: Verify the full build and run the complete test suite

**Files:** None (verification only)

- [ ] **Step 5.1: Build the binary**

```
go build ./...
```

Expected: no errors

- [ ] **Step 5.2: Run the full test suite**

```
go test ./... -count=1
```

Expected: all PASS, no skips

- [ ] **Step 5.3: Smoke-test `cleo init` output (manual)**

```
./bin/cleo init --help
```

Expected: `--force` and `--yes` flags listed

```
echo "y\nn\nn\ny\n" | ./bin/cleo init
```

Expected: Claude and OpenCode are selected, plugin written to `~/.config/opencode/plugins/cleo.ts`, output shows "7 events" for OpenCode

- [ ] **Step 5.4: Smoke-test `cleo doctor` output (manual)**

```
./bin/cleo doctor
```

Expected: "OpenCode plugin" row visible, either OK (if plugin was installed) or showing the install hint

- [ ] **Step 5.5: Final commit**

```bash
git add -p  # stage any stray changes
git commit -m "chore: opencode hook integration — complete"
```

---

## Self-Review Checklist

**Spec coverage:**
- ✅ `session.created` → `EvSessionStart` + `session_start` (Task 1)
- ✅ `tool.execute.before` → `EvPreToolUse` (Task 1)
- ✅ `tool.execute.after` → `EvPostToolUse` (Task 1)
- ✅ `permission.asked` → `EvNotification` + `needs_input`, `SuppressWhenIdle: false` (Task 1)
- ✅ `session.idle` → `EvStop` + `session_idle` (Task 1)
- ✅ `session.deleted` → `EvSessionEnd` + `session_completed` (Task 1)
- ✅ `session.error` → `EvError` + `session_error` (Task 1)
- ✅ Plugin template written to `~/.config/opencode/plugins/cleo.ts` (Task 1)
- ✅ `cleo init` installs plugin, shows in selection prompt (Task 3)
- ✅ `cleo doctor` reports plugin status and hook activity (Task 4)
- ✅ Defaults updated: `Hooks: "opencode"` (Task 3)
- ✅ Protocol registered in `Protocols()` (Task 2)
- ✅ `cleo init --force` overwrites existing plugin (Task 2, Install_ForceOverwrites test)
- ✅ `cleo cleanup` not broken (Cleanup tested in Task 2)
- ✅ `ExpectedOpenCodeEntry()` for doctor diff (Task 1)

**Type consistency:** `OpenCodeProtocol`, `OpenCodeEvents()`, `ExpectedOpenCodeEntry()`, `openCodePluginsDir`, `openCodePlugDir()`, `openCodePluginTemplate` — all consistent across tasks.

**No placeholders:** All steps have complete code blocks.
