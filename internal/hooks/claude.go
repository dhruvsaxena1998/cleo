package hooks

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/dhruvsaxena1998/cleo/internal/state"
)

// ClaudeEvents returns the hook event names Claude Code fires.
func (ClaudeProtocol) Name() string          { return "claude" }
func (ClaudeProtocol) Events() []string      { return ClaudeEvents() }
func (ClaudeProtocol) UsesCwdFallback() bool { return false }

func (ClaudeProtocol) Install(cleoBin string, force bool) error {
	home, _ := os.UserHomeDir()
	return InstallClaude(filepath.Join(home, ".claude", "settings.json"), cleoBin, force)
}

func (ClaudeProtocol) Cleanup() error {
	home, _ := os.UserHomeDir()
	_, err := CleanupClaude(filepath.Join(home, ".claude", "settings.json"))
	return err
}

func (ClaudeProtocol) Normalize(event string, payload []byte) (NormalizedEvent, bool) {
	var p struct {
		ToolName string `json:"tool_name"`
		Message  string `json:"message"`
	}
	_ = json.Unmarshal(payload, &p)

	switch event {
	case "SessionStart":
		return NormalizedEvent{StateEvent: state.EvSessionStart, SoundEvent: "session_start", ToolName: p.ToolName}, true
	case "UserPromptSubmit":
		return NormalizedEvent{StateEvent: state.EvUserResume, ToolName: p.ToolName}, true
	case "PreToolUse":
		return NormalizedEvent{StateEvent: state.EvPreToolUse, ToolName: p.ToolName}, true
	case "PostToolUse":
		return NormalizedEvent{StateEvent: state.EvPostToolUse, ToolName: p.ToolName}, true
	case "Notification":
		return NormalizedEvent{StateEvent: state.EvNotification, SoundEvent: "needs_input", Message: p.Message, ToolName: p.ToolName}, true
	case "Stop":
		return NormalizedEvent{StateEvent: state.EvStop, SoundEvent: "session_idle", ToolName: p.ToolName}, true
	case "SessionEnd":
		return NormalizedEvent{StateEvent: state.EvSessionEnd, SoundEvent: "session_completed", ToolName: p.ToolName}, true
	case "SubagentStop":
		return NormalizedEvent{LogOnly: true, LogType: "SubagentStop", ToolName: p.ToolName}, true
	}
	return NormalizedEvent{}, false
}
