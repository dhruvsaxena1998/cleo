package hooks

import (
	"fmt"
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
