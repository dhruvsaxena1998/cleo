package hooks

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// codexEvents are the hook events cleo observes from Codex CLI.
// Codex uses the same PascalCase names as Claude Code for lifecycle hooks.
// PermissionRequest is the codex equivalent of Notification (approval needed).
// UserPromptSubmit is important because it marks an idle session as running
// as soon as the user starts a new turn, before the first tool call.
var codexEvents = []string{
	"SessionStart", "UserPromptSubmit", "PreToolUse", "PostToolUse",
	"PermissionRequest", "Stop",
}

// CodexEvents returns the hook event names Codex fires.
func CodexEvents() []string { return append([]string(nil), codexEvents...) }

func (CodexProtocol) Name() string          { return "codex" }
func (CodexProtocol) Events() []string      { return CodexEvents() }
func (CodexProtocol) UsesCwdFallback() bool { return true }

func (CodexProtocol) Install(cleoBin string, force bool) error {
	home, _ := os.UserHomeDir()
	return InstallCodex(
		filepath.Join(home, ".codex", "hooks.json"),
		filepath.Join(home, ".codex", "config.toml"),
		cleoBin, force,
	)
}

func (CodexProtocol) Cleanup() error {
	home, _ := os.UserHomeDir()
	_, err := CleanupCodex(filepath.Join(home, ".codex", "hooks.json"))
	return err
}

func (CodexProtocol) Normalize(event string, payload []byte) (NormalizedEvent, bool) {
	if event == "PermissionRequest" {
		norm, ok := ClaudeProtocol{}.Normalize("Notification", synthesizeNotification(payload))
		if ok {
			// Codex PermissionRequest is a genuine blocking request, not an idle nudge.
			// Unlike Claude's Notification (which may be an idle-nudge timer),
			// PermissionRequest must play sound even when the session is Idle.
			norm.SuppressWhenIdle = false
		}
		return norm, ok
	}
	return ClaudeProtocol{}.Normalize(event, payload)
}

// synthesizeNotification builds a Claude-payload-shaped JSON from a Codex
// PermissionRequest payload. Preference order: command > description > tool_name.
func synthesizeNotification(payload []byte) []byte {
	var pr struct {
		ToolName  string `json:"tool_name"`
		ToolInput struct {
			Command     string `json:"command"`
			Description string `json:"description"`
		} `json:"tool_input"`
	}
	_ = json.Unmarshal(payload, &pr)
	msg := pr.ToolName
	if pr.ToolInput.Command != "" {
		msg = pr.ToolInput.Command
	} else if pr.ToolInput.Description != "" {
		msg = pr.ToolInput.Description
	}
	out, _ := json.Marshal(struct {
		ToolName string `json:"tool_name"`
		Message  string `json:"message"`
	}{pr.ToolName, msg})
	return out
}
