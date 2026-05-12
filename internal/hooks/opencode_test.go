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
