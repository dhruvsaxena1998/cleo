package hooks

import (
	"fmt"
	"os"
	"path/filepath"
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

func TestPiProtocol_Install_WritesExtension(t *testing.T) {
	dir := t.TempDir()
	extDir := filepath.Join(dir, ".pi", "agent", "extensions")

	setTestHome(t, dir)

	proto := PiProtocol{}
	if _, err := proto.Install("/usr/local/bin/cleo", false); err != nil {
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

func TestPiProtocol_Install_Idempotent(t *testing.T) {
	dir := t.TempDir()
	setTestHome(t, dir)

	proto := PiProtocol{}
	if _, err := proto.Install("/usr/local/bin/cleo", false); err != nil {
		t.Fatal(err)
	}
	// Re-install with same content — must not fail.
	if _, err := proto.Install("/usr/local/bin/cleo", false); err != nil {
		t.Errorf("re-install with same content should not fail: %v", err)
	}
}

func TestPiProtocol_Install_ConflictWithoutForce(t *testing.T) {
	dir := t.TempDir()
	extDir := filepath.Join(dir, ".pi", "agent", "extensions")
	setTestHome(t, dir)

	// Write a different file first.
	_ = os.MkdirAll(extDir, 0o755)
	_ = os.WriteFile(filepath.Join(extDir, "cleo.ts"), []byte("// different content"), 0o644)

	proto := PiProtocol{}
	if _, err := proto.Install("/usr/local/bin/cleo", false); err == nil {
		t.Error("expected conflict error, got nil")
	}
}

func TestPiProtocol_Install_ForceOverwrites(t *testing.T) {
	dir := t.TempDir()
	extDir := filepath.Join(dir, ".pi", "agent", "extensions")
	setTestHome(t, dir)

	_ = os.MkdirAll(extDir, 0o755)
	_ = os.WriteFile(filepath.Join(extDir, "cleo.ts"), []byte("// different content"), 0o644)

	proto := PiProtocol{}
	if _, err := proto.Install("/usr/local/bin/cleo", true); err != nil {
		t.Fatalf("Install with --force: %v", err)
	}
	got, _ := os.ReadFile(filepath.Join(extDir, "cleo.ts"))
	if string(got) != piExtensionTemplate {
		t.Error("force install did not overwrite with template")
	}
}

func TestPiProtocol_Cleanup_RemovesMatchingFile(t *testing.T) {
	dir := t.TempDir()
	extDir := filepath.Join(dir, ".pi", "agent", "extensions")
	setTestHome(t, dir)

	proto := PiProtocol{}
	_, _ = proto.Install("/usr/local/bin/cleo", false)

	dest := filepath.Join(extDir, "cleo.ts")
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

func TestPiProtocol_Cleanup_SkipsModifiedFile(t *testing.T) {
	dir := t.TempDir()
	extDir := filepath.Join(dir, ".pi", "agent", "extensions")
	setTestHome(t, dir)

	_ = os.MkdirAll(extDir, 0o755)
	dest := filepath.Join(extDir, "cleo.ts")
	_ = os.WriteFile(dest, []byte("// user-modified"), 0o644)

	proto := PiProtocol{}
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

func TestPiProtocol_Cleanup_MissingWhenFileAbsent(t *testing.T) {
	dir := t.TempDir()
	setTestHome(t, dir)

	proto := PiProtocol{}
	outcome, err := proto.Cleanup()
	if err != nil {
		t.Fatal(err)
	}
	if outcome.Status != CleanupStatusMissing {
		t.Errorf("Status = %v, want CleanupStatusMissing", outcome.Status)
	}
}

func TestExpectedPiEntry_MatchesTemplate(t *testing.T) {
	if ExpectedPiEntry() != piExtensionTemplate {
		t.Error("ExpectedPiEntry() must return the embedded template")
	}
}
