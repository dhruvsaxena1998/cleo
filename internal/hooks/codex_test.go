package hooks

import (
	"encoding/json"
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
			result := synthesizeNotification([]byte(tt.payload))
			var p struct {
				Message string `json:"message"`
			}
			if err := json.Unmarshal(result, &p); err != nil {
				t.Fatalf("unmarshal result: %v", err)
			}
			if p.Message != tt.wantMsg {
				t.Errorf("message = %q, want %q", p.Message, tt.wantMsg)
			}
		})
	}
}
