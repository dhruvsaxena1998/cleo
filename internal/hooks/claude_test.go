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
