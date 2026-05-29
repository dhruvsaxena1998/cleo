package hooks

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
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
func (CodexProtocol) DisplayName() string   { return "Codex" }
func (CodexProtocol) Events() []string      { return CodexEvents() }
func (CodexProtocol) UsesCwdFallback() bool { return true }

func (CodexProtocol) hooksPath() string  { return filepath.Join(homeDir(), ".codex", "hooks.json") }
func (CodexProtocol) configPath() string { return filepath.Join(homeDir(), ".codex", "config.toml") }

func (p CodexProtocol) Locations() []Location {
	return []Location{
		{Label: "hooks", Path: p.hooksPath()},
		{Label: "feature flag", Path: p.configPath()},
	}
}

func (p CodexProtocol) Install(cleoBin string, force bool) (InstallReport, error) {
	if err := InstallCodex(p.hooksPath(), p.configPath(), cleoBin, force); err != nil {
		return InstallReport{}, err
	}
	// Codex gates hooks behind in-app approval — the user must run /hooks and
	// approve the cleo entries before they take effect.
	return InstallReport{ManualReview: &ReviewStep{
		Command: fmt.Sprintf("%s hooks invoke codex", cleoBin),
		Hooks:   CodexEvents(),
	}}, nil
}

func (p CodexProtocol) Cleanup() (CleanupOutcome, error) {
	outcome, err := CleanupCodex(p.hooksPath())
	if err != nil {
		return outcome, err
	}
	outcome.Notes = append(outcome.Notes,
		"left ~/.codex/config.toml [features].hooks unchanged; that flag may be used by other Codex hooks")
	return outcome, nil
}

func (p CodexProtocol) Diagnose() []Check {
	return []Check{
		diagnoseCodexFeatureFlag(p.configPath()),
		diagnoseJSONHooks("Codex hooks", p.hooksPath(), CodexEvents(), "hooks invoke codex"),
	}
}

// diagnoseCodexFeatureFlag checks ~/.codex/config.toml has [features].hooks = true
// and no deprecated codex_hooks flag.
func diagnoseCodexFeatureFlag(path string) Check {
	const label = "Codex feature flag"
	b, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return Check{Label: label, Detail: fmt.Sprintf("missing %s; run cleo hooks init", path)}
	}
	if err != nil {
		return Check{Label: label, Detail: err.Error()}
	}
	content := string(b)
	if strings.Contains(content, "codex_hooks") {
		return Check{Label: label, Detail: fmt.Sprintf("deprecated codex_hooks flag found in %s; run cleo hooks init", path)}
	}
	if !strings.Contains(content, "hooks = true") {
		return Check{Label: label, Detail: fmt.Sprintf("[features].hooks = true not found in %s; run cleo hooks init", path)}
	}
	return Check{Label: label, OK: true, Detail: "[features].hooks = true"}
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
