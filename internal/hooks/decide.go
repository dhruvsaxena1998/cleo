package hooks

import (
	"github.com/dhruvsaxena1998/cleo/internal/events"
	"github.com/dhruvsaxena1998/cleo/internal/state"
)

// soundReason categorises why a sound did or didn't play for a hook event.
// It is recorded in the hook trace log for the doctor to read back.
type soundReason string

const (
	soundPlayed    soundReason = "played"
	soundDisabled  soundReason = "disabled"
	soundFocus     soundReason = "focus"
	soundIdleNudge soundReason = "idle-nudge"
)

// hookOutcome is the Hook outcome: the complete set of effects a normalized
// hook event produces — the state transition, the event-log entry, and the
// sound decision. decideHook computes it purely; applyNormalized performs it.
type hookOutcome struct {
	Transition  state.Event  // "" => no state transition (LogOnly, or no StateEvent)
	Message     string       // passed to State.Apply alongside Transition
	LogEntry    events.Entry // always appended to the event log
	SoundEvent  string       // "" => no sound considered or logged
	SoundReason soundReason  // why the sound did/didn't play; logged for the doctor
	PlaySound   bool         // true only when SoundReason == soundPlayed
}

// decideHook is pure: given the normalized event, the pre-transition session
// state, and whether the sound event is enabled / the session is focused, it
// returns the effects to apply. No IO, no Deps — the interface is the test
// surface.
func decideHook(norm NormalizedEvent, fromState state.State, soundEnabled, focused bool) hookOutcome {
	out := hookOutcome{
		Message:    norm.Message,
		LogEntry:   events.Entry{Type: logType(norm), Tool: norm.ToolName, Detail: norm.Message},
		SoundEvent: norm.SoundEvent,
	}
	if !norm.LogOnly {
		out.Transition = norm.StateEvent
	}
	if norm.SoundEvent != "" {
		switch {
		case !soundEnabled:
			out.SoundReason = soundDisabled
		case focused:
			out.SoundReason = soundFocus
		case norm.SuppressWhenIdle && fromState == state.Idle:
			out.SoundReason = soundIdleNudge
		default:
			out.SoundReason = soundPlayed
			out.PlaySound = true
		}
	}
	return out
}

// logType is the event-log entry type for a normalized event: the explicit
// LogType override when set, otherwise the state event name.
func logType(norm NormalizedEvent) string {
	if norm.LogType != "" {
		return norm.LogType
	}
	return string(norm.StateEvent)
}
