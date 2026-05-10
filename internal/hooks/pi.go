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
