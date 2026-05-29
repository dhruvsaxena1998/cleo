package hooks

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/dhruvsaxena1998/cleo/internal/state"
)

func TestOpenCodeProtocol_Normalize(t *testing.T) {
	proto := OpenCodeProtocol{}

	tests := []struct {
		name    string
		event   string
		payload string
		want    NormalizedEvent
		wantOk  bool
	}{
		{
			name:    "session.created",
			event:   "session.created",
			payload: `{"cwd":"/proj","session_id":"cleo-x-opencode-1"}`,
			want:    NormalizedEvent{StateEvent: state.EvSessionStart, SoundEvent: "session_start"},
			wantOk:  true,
		},
		{
			name:    "tool.execute.before",
			event:   "tool.execute.before",
			payload: `{"cwd":"/proj","session_id":"cleo-x-opencode-1","tool_name":"write"}`,
			want:    NormalizedEvent{StateEvent: state.EvPreToolUse, ToolName: "write"},
			wantOk:  true,
		},
		{
			name:    "tool.execute.before/question",
			event:   "tool.execute.before",
			payload: `{"cwd":"/proj","session_id":"cleo-x-opencode-1","tool_name":"question"}`,
			want:    NormalizedEvent{StateEvent: state.EvNotification, SoundEvent: "needs_input", ToolName: "question"},
			wantOk:  true,
		},
		{
			name:    "tool.execute.after",
			event:   "tool.execute.after",
			payload: `{"cwd":"/proj","session_id":"cleo-x-opencode-1","tool_name":"write"}`,
			want:    NormalizedEvent{StateEvent: state.EvPostToolUse, ToolName: "write"},
			wantOk:  true,
		},
		{
			name:    "permission.asked",
			event:   "permission.asked",
			payload: `{"cwd":"/proj","session_id":"cleo-x-opencode-1"}`,
			want:    NormalizedEvent{StateEvent: state.EvNotification, SoundEvent: "needs_input", SuppressWhenIdle: false},
			wantOk:  true,
		},
		{
			name:    "session.idle",
			event:   "session.idle",
			payload: `{"cwd":"/proj","session_id":"cleo-x-opencode-1"}`,
			want:    NormalizedEvent{StateEvent: state.EvStop, SoundEvent: "session_idle"},
			wantOk:  true,
		},
		{
			name:    "session.deleted",
			event:   "session.deleted",
			payload: `{"cwd":"/proj","session_id":"cleo-x-opencode-1"}`,
			want:    NormalizedEvent{StateEvent: state.EvSessionEnd, SoundEvent: "session_completed"},
			wantOk:  true,
		},
		{
			name:    "session.error",
			event:   "session.error",
			payload: `{"cwd":"/proj","session_id":"cleo-x-opencode-1"}`,
			want:    NormalizedEvent{StateEvent: state.EvError, SoundEvent: "session_error"},
			wantOk:  true,
		},
		{
			name:    "unknown_event",
			event:   "unknown_event",
			payload: `{}`,
			wantOk:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
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

func TestOpenCodeProtocol_Install_WritesPlugin(t *testing.T) {
	dir := t.TempDir()
	plugDir := filepath.Join(dir, ".config", "opencode", "plugins")
	setTestHome(t, dir)

	proto := OpenCodeProtocol{}
	if _, err := proto.Install("/usr/local/bin/cleo", false); err != nil {
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
	setTestHome(t, dir)

	proto := OpenCodeProtocol{}
	if _, err := proto.Install("/usr/local/bin/cleo", false); err != nil {
		t.Fatal(err)
	}
	if _, err := proto.Install("/usr/local/bin/cleo", false); err != nil {
		t.Errorf("re-install with same content should not fail: %v", err)
	}
}

func TestOpenCodeProtocol_Install_ConflictWithoutForce(t *testing.T) {
	dir := t.TempDir()
	plugDir := filepath.Join(dir, ".config", "opencode", "plugins")
	setTestHome(t, dir)

	_ = os.MkdirAll(plugDir, 0o755)
	_ = os.WriteFile(filepath.Join(plugDir, "cleo.ts"), []byte("// different content"), 0o644)

	proto := OpenCodeProtocol{}
	if _, err := proto.Install("/usr/local/bin/cleo", false); err == nil {
		t.Error("expected conflict error, got nil")
	}
}

func TestOpenCodeProtocol_Install_ForceOverwrites(t *testing.T) {
	dir := t.TempDir()
	plugDir := filepath.Join(dir, ".config", "opencode", "plugins")
	setTestHome(t, dir)

	_ = os.MkdirAll(plugDir, 0o755)
	_ = os.WriteFile(filepath.Join(plugDir, "cleo.ts"), []byte("// different content"), 0o644)

	proto := OpenCodeProtocol{}
	if _, err := proto.Install("/usr/local/bin/cleo", true); err != nil {
		t.Fatalf("Install with --force: %v", err)
	}
	got, _ := os.ReadFile(filepath.Join(plugDir, "cleo.ts"))
	if string(got) != openCodePluginTemplate {
		t.Error("force install did not overwrite with template")
	}
}

func TestOpenCodeProtocol_Cleanup_RemovesMatchingFile(t *testing.T) {
	dir := t.TempDir()
	plugDir := filepath.Join(dir, ".config", "opencode", "plugins")
	setTestHome(t, dir)

	proto := OpenCodeProtocol{}
	_, _ = proto.Install("/usr/local/bin/cleo", false)

	dest := filepath.Join(plugDir, "cleo.ts")
	outcome, err := proto.Cleanup()
	if err != nil {
		t.Fatalf("Cleanup: %v", err)
	}
	if outcome.Status != CleanupStatusRemoved {
		t.Errorf("Status = %v, want CleanupStatusRemoved", outcome.Status)
	}
	if outcome.Path != dest {
		t.Errorf("Path = %q, want %q", outcome.Path, dest)
	}
	if _, err := os.Stat(dest); !os.IsNotExist(err) {
		t.Error("expected cleo.ts to be removed after Cleanup")
	}
}

func TestOpenCodeProtocol_Cleanup_SkipsModifiedFile(t *testing.T) {
	dir := t.TempDir()
	plugDir := filepath.Join(dir, ".config", "opencode", "plugins")
	setTestHome(t, dir)

	_ = os.MkdirAll(plugDir, 0o755)
	dest := filepath.Join(plugDir, "cleo.ts")
	_ = os.WriteFile(dest, []byte("// user-modified"), 0o644)

	proto := OpenCodeProtocol{}
	outcome, err := proto.Cleanup()
	if err != nil {
		t.Fatalf("Cleanup returned error for modified file: %v", err)
	}
	if outcome.Status != CleanupStatusSkippedModified {
		t.Errorf("Status = %v, want CleanupStatusSkippedModified", outcome.Status)
	}
	if outcome.Path != dest {
		t.Errorf("Path = %q, want %q", outcome.Path, dest)
	}
	if _, err := os.Stat(dest); os.IsNotExist(err) {
		t.Error("Cleanup must NOT remove a user-modified file")
	}
}

func TestOpenCodeProtocol_Cleanup_MissingWhenFileAbsent(t *testing.T) {
	dir := t.TempDir()
	setTestHome(t, dir)

	proto := OpenCodeProtocol{}
	outcome, err := proto.Cleanup()
	if err != nil {
		t.Fatal(err)
	}
	if outcome.Status != CleanupStatusMissing {
		t.Errorf("Status = %v, want CleanupStatusMissing", outcome.Status)
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
