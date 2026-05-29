package hooks

import (
	"testing"

	"github.com/dhruvsaxena1998/cleo/internal/state"
)

// decideHook is pure, so its whole behaviour is a table: given a normalized
// event plus the pre-transition state and the two precomputed bools, assert
// the outcome. No temp dirs, no config.Load, no fakes.
func TestDecideHook(t *testing.T) {
	tests := []struct {
		name         string
		norm         NormalizedEvent
		fromState    state.State
		soundEnabled bool
		focused      bool

		wantTransition  state.Event
		wantLogType     string
		wantSoundEvent  string
		wantSoundReason soundReason
		wantPlay        bool
	}{
		{
			name:            "enabled unfocused event plays",
			norm:            NormalizedEvent{StateEvent: state.EvSessionEnd, SoundEvent: "session_completed"},
			fromState:       state.Running,
			soundEnabled:    true,
			wantTransition:  state.EvSessionEnd,
			wantLogType:     string(state.EvSessionEnd),
			wantSoundEvent:  "session_completed",
			wantSoundReason: soundPlayed,
			wantPlay:        true,
		},
		{
			name:            "disabled sound event does not play",
			norm:            NormalizedEvent{StateEvent: state.EvSessionEnd, SoundEvent: "session_completed"},
			fromState:       state.Running,
			soundEnabled:    false,
			wantTransition:  state.EvSessionEnd,
			wantLogType:     string(state.EvSessionEnd),
			wantSoundEvent:  "session_completed",
			wantSoundReason: soundDisabled,
			wantPlay:        false,
		},
		{
			name:            "focused session suppresses sound",
			norm:            NormalizedEvent{StateEvent: state.EvSessionEnd, SoundEvent: "session_completed"},
			fromState:       state.Running,
			soundEnabled:    true,
			focused:         true,
			wantTransition:  state.EvSessionEnd,
			wantLogType:     string(state.EvSessionEnd),
			wantSoundEvent:  "session_completed",
			wantSoundReason: soundFocus,
			wantPlay:        false,
		},
		{
			name:            "idle-nudge: SuppressWhenIdle from Idle does not play",
			norm:            NormalizedEvent{StateEvent: state.EvNotification, SoundEvent: "needs_input", SuppressWhenIdle: true},
			fromState:       state.Idle,
			soundEnabled:    true,
			wantTransition:  state.EvNotification,
			wantLogType:     string(state.EvNotification),
			wantSoundEvent:  "needs_input",
			wantSoundReason: soundIdleNudge,
			wantPlay:        false,
		},
		{
			name:            "genuine notification: SuppressWhenIdle from Running plays",
			norm:            NormalizedEvent{StateEvent: state.EvNotification, SoundEvent: "needs_input", SuppressWhenIdle: true},
			fromState:       state.Running,
			soundEnabled:    true,
			wantTransition:  state.EvNotification,
			wantLogType:     string(state.EvNotification),
			wantSoundEvent:  "needs_input",
			wantSoundReason: soundPlayed,
			wantPlay:        true,
		},
		{
			name:            "SuppressWhenIdle=false plays even from Idle",
			norm:            NormalizedEvent{StateEvent: state.EvNotification, SoundEvent: "needs_input", SuppressWhenIdle: false},
			fromState:       state.Idle,
			soundEnabled:    true,
			wantTransition:  state.EvNotification,
			wantLogType:     string(state.EvNotification),
			wantSoundEvent:  "needs_input",
			wantSoundReason: soundPlayed,
			wantPlay:        true,
		},
		{
			name:            "no sound event: no sound considered",
			norm:            NormalizedEvent{StateEvent: state.EvPreToolUse, ToolName: "Bash"},
			fromState:       state.Running,
			soundEnabled:    true,
			wantTransition:  state.EvPreToolUse,
			wantLogType:     string(state.EvPreToolUse),
			wantSoundEvent:  "",
			wantSoundReason: "",
			wantPlay:        false,
		},
		{
			name:            "LogOnly: no transition, LogType drives entry",
			norm:            NormalizedEvent{LogOnly: true, LogType: "SubagentStop", ToolName: "Task"},
			fromState:       state.Running,
			soundEnabled:    true,
			wantTransition:  "",
			wantLogType:     "SubagentStop",
			wantSoundEvent:  "",
			wantSoundReason: "",
			wantPlay:        false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out := decideHook(tt.norm, tt.fromState, tt.soundEnabled, tt.focused)

			if out.Transition != tt.wantTransition {
				t.Errorf("Transition = %q, want %q", out.Transition, tt.wantTransition)
			}
			if out.LogEntry.Type != tt.wantLogType {
				t.Errorf("LogEntry.Type = %q, want %q", out.LogEntry.Type, tt.wantLogType)
			}
			if out.LogEntry.Tool != tt.norm.ToolName {
				t.Errorf("LogEntry.Tool = %q, want %q", out.LogEntry.Tool, tt.norm.ToolName)
			}
			if out.LogEntry.Detail != tt.norm.Message {
				t.Errorf("LogEntry.Detail = %q, want %q", out.LogEntry.Detail, tt.norm.Message)
			}
			if out.SoundEvent != tt.wantSoundEvent {
				t.Errorf("SoundEvent = %q, want %q", out.SoundEvent, tt.wantSoundEvent)
			}
			if out.SoundReason != tt.wantSoundReason {
				t.Errorf("SoundReason = %q, want %q", out.SoundReason, tt.wantSoundReason)
			}
			if out.PlaySound != tt.wantPlay {
				t.Errorf("PlaySound = %v, want %v", out.PlaySound, tt.wantPlay)
			}
		})
	}
}

func TestDecideHook_MessagePassedToTransition(t *testing.T) {
	norm := NormalizedEvent{StateEvent: state.EvNotification, Message: "Approve Bash command?", SoundEvent: "needs_input", SuppressWhenIdle: true}

	out := decideHook(norm, state.Running, true, false)

	if out.Message != "Approve Bash command?" {
		t.Errorf("Message = %q, want the notification text", out.Message)
	}
	if out.LogEntry.Detail != "Approve Bash command?" {
		t.Errorf("LogEntry.Detail = %q, want the notification text", out.LogEntry.Detail)
	}
}
