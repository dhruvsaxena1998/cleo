package hooks

import (
	"encoding/json"
	"path/filepath"

	"github.com/dhruvsaxena1998/cleo/internal/state"
)

func (ClaudeProtocol) Name() string          { return "claude" }
func (ClaudeProtocol) DisplayName() string   { return "Claude Code" }
func (ClaudeProtocol) Events() []string      { return ClaudeEvents() }
func (ClaudeProtocol) UsesCwdFallback() bool { return false }

func (ClaudeProtocol) settingsPath() string {
	return filepath.Join(homeDir(), ".claude", "settings.json")
}

func (p ClaudeProtocol) Locations() []Location {
	return []Location{{Label: "hooks", Path: p.settingsPath()}}
}

func (p ClaudeProtocol) Install(cleoBin string, force bool) (InstallReport, error) {
	if err := InstallClaude(p.settingsPath(), cleoBin, force); err != nil {
		return InstallReport{}, err
	}
	return InstallReport{}, nil
}

func (p ClaudeProtocol) Cleanup() (CleanupOutcome, error) {
	return CleanupClaude(p.settingsPath())
}

func (p ClaudeProtocol) Diagnose() []Check {
	return []Check{diagnoseJSONHooks("Claude hooks", p.settingsPath(), ClaudeEvents(), "hooks invoke claude")}
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
		return NormalizedEvent{StateEvent: state.EvNotification, SoundEvent: "needs_input", Message: p.Message, ToolName: p.ToolName, SuppressWhenIdle: true}, true
	case "Stop":
		return NormalizedEvent{StateEvent: state.EvStop, SoundEvent: "session_idle", ToolName: p.ToolName}, true
	case "SessionEnd":
		return NormalizedEvent{StateEvent: state.EvSessionEnd, SoundEvent: "session_completed", ToolName: p.ToolName}, true
	case "SubagentStop":
		return NormalizedEvent{LogOnly: true, LogType: "SubagentStop", ToolName: p.ToolName}, true
	}
	return NormalizedEvent{}, false
}
