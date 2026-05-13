package hooks

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
)

var claudeEvents = []string{"SessionStart", "UserPromptSubmit", "PreToolUse", "PostToolUse", "Notification", "Stop", "SessionEnd", "SubagentStop"}

func ClaudeEvents() []string {
	return append([]string(nil), claudeEvents...)
}

// ExpectedClaudeEntries returns the per-event hook entries that
// InstallClaude would write for the given cleo binary path. Keyed by
// Claude hook event name (SessionStart, PreToolUse, …). The values match
// the on-disk JSON shape expected by Claude Code.
func ExpectedClaudeEntries(cleoBin string) map[string]any {
	out := make(map[string]any, len(claudeEvents))
	for _, ev := range claudeEvents {
		out[ev] = []any{
			map[string]any{
				"hooks": []any{
					map[string]any{
						"type":    "command",
						"command": fmt.Sprintf("%s hook claude %s", cleoBin, ev),
						"timeout": 5,
					},
				},
			},
		}
	}
	return out
}

func InstallClaude(settingsPath, cleoBin string, force bool) error {
	b, err := os.ReadFile(settingsPath)
	if errors.Is(err, os.ErrNotExist) {
		b = []byte("{}")
	} else if err != nil {
		return err
	}
	var settings map[string]any
	if err := json.Unmarshal(b, &settings); err != nil {
		return fmt.Errorf("settings.json: %w", err)
	}
	hooks, _ := settings["hooks"].(map[string]any)
	if hooks == nil {
		hooks = map[string]any{}
	}
	expected := ExpectedClaudeEntries(cleoBin)
	for _, ev := range claudeEvents {
		want := expected[ev]
		cmd := fmt.Sprintf("%s hook claude %s", cleoBin, ev)
		if hookCommandPresent(hooks[ev], cmd) {
			continue // already installed — skip, don't overwrite
		}
		if existing, ok := hooks[ev]; ok {
			if !equalsHook(existing, want) && !force {
				return fmt.Errorf("conflict: %s already has a different hook (re-run with --force to overwrite)", ev)
			}
		}
		hooks[ev] = want
	}
	settings["hooks"] = hooks
	out, _ := json.MarshalIndent(settings, "", "  ")
	return os.WriteFile(settingsPath, out, 0o644)
}

func CleanupClaude(settingsPath string) (int, error) {
	return cleanupHookFile(settingsPath, "claude", "settings.json")
}

// ExpectedCodexEntries returns the per-event hook entries that
// InstallCodex would write for the given cleo binary path. Keyed by
// Codex hook event name (SessionStart, PreToolUse, …).
func ExpectedCodexEntries(cleoBin string) map[string]any {
	out := make(map[string]any, len(codexEvents))
	for _, ev := range codexEvents {
		out[ev] = []any{
			map[string]any{
				"hooks": []any{
					map[string]any{
						"type":    "command",
						"command": fmt.Sprintf("%s hook codex %s", cleoBin, ev),
						"timeout": 5,
					},
				},
			},
		}
	}
	return out
}

// InstallCodex writes hook entries to hooksPath (~/.codex/hooks.json) and
// ensures the feature flag is set in configPath (~/.codex/config.toml).
func InstallCodex(hooksPath, configPath, cleoBin string, force bool) error {
	if err := os.MkdirAll(dirOf(hooksPath), 0o755); err != nil {
		return err
	}
	b, err := os.ReadFile(hooksPath)
	if errors.Is(err, os.ErrNotExist) {
		b = []byte("{}")
	} else if err != nil {
		return err
	}
	var settings map[string]any
	if err := json.Unmarshal(b, &settings); err != nil {
		return fmt.Errorf("hooks.json: %w", err)
	}
	hooks, _ := settings["hooks"].(map[string]any)
	if hooks == nil {
		hooks = map[string]any{}
	}
	expected := ExpectedCodexEntries(cleoBin)
	for _, ev := range codexEvents {
		want := expected[ev]
		cmd := fmt.Sprintf("%s hook codex %s", cleoBin, ev)
		if hookCommandPresent(hooks[ev], cmd) {
			continue // already installed — skip, don't overwrite
		}
		if existing, ok := hooks[ev]; ok {
			if !equalsHook(existing, want) && !force {
				return fmt.Errorf("conflict: %s already has a different hook (re-run with --force to overwrite)", ev)
			}
		}
		hooks[ev] = want
	}
	settings["hooks"] = hooks
	out, _ := json.MarshalIndent(settings, "", "  ")
	if err := os.WriteFile(hooksPath, out, 0o644); err != nil {
		return err
	}
	return ensureCodexFeatureFlag(configPath)
}

func CleanupCodex(hooksPath string) (int, error) {
	return cleanupHookFile(hooksPath, "codex", "hooks.json")
}

func cleanupHookFile(path, protocol, label string) (int, error) {
	b, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return 0, nil
	} else if err != nil {
		return 0, err
	}
	var settings map[string]any
	if err := json.Unmarshal(b, &settings); err != nil {
		return 0, fmt.Errorf("%s: %w", label, err)
	}
	removed := removeProtocolHooks(settings, protocol)
	if removed == 0 {
		return 0, nil
	}
	out, _ := json.MarshalIndent(settings, "", "  ")
	return removed, os.WriteFile(path, out, 0o644)
}

func removeProtocolHooks(settings map[string]any, protocol string) int {
	hooksMap, _ := settings["hooks"].(map[string]any)
	if hooksMap == nil {
		return 0
	}
	removed := 0
	for event, rawEntries := range hooksMap {
		entries, ok := rawEntries.([]any)
		if !ok {
			continue
		}
		cleanedEntries := make([]any, 0, len(entries))
		for _, rawEntry := range entries {
			entry, ok := rawEntry.(map[string]any)
			if !ok {
				cleanedEntries = append(cleanedEntries, rawEntry)
				continue
			}
			rawHooks, ok := entry["hooks"].([]any)
			if !ok {
				cleanedEntries = append(cleanedEntries, rawEntry)
				continue
			}
			cleanedHooks := make([]any, 0, len(rawHooks))
			for _, rawHook := range rawHooks {
				if isCleoHook(rawHook, protocol) {
					removed++
					continue
				}
				cleanedHooks = append(cleanedHooks, rawHook)
			}
			if len(cleanedHooks) == 0 {
				continue
			}
			entry["hooks"] = cleanedHooks
			cleanedEntries = append(cleanedEntries, entry)
		}
		if len(cleanedEntries) == 0 {
			delete(hooksMap, event)
			continue
		}
		hooksMap[event] = cleanedEntries
	}
	if len(hooksMap) == 0 {
		delete(settings, "hooks")
	}
	return removed
}

func isCleoHook(rawHook any, protocol string) bool {
	hook, ok := rawHook.(map[string]any)
	if !ok {
		return false
	}
	command, _ := hook["command"].(string)
	return strings.Contains(command, " hook "+protocol+" ")
}

// ensureCodexFeatureFlag adds `hooks = true` under [features] in the
// codex config.toml if not already present.
func ensureCodexFeatureFlag(configPath string) error {
	b, err := os.ReadFile(configPath)
	if errors.Is(err, os.ErrNotExist) {
		b = nil
	} else if err != nil {
		return err
	}
	content := string(b)
	if strings.Contains(content, "hooks = true") && !strings.Contains(content, "codex_hooks") {
		return nil // already set
	}
	// Append the feature flag. If a [features] section already exists but
	// lacks the key, append after it; otherwise add a new section.
	var out strings.Builder
	foundFeatures := false
	scanner := bufio.NewScanner(strings.NewReader(content))
	for scanner.Scan() {
		line := scanner.Text()
		if strings.TrimSpace(line) == "codex_hooks = true" {
			line = "hooks = true"
		}
		out.WriteString(line + "\n")
		if strings.TrimSpace(line) == "[features]" {
			foundFeatures = true
			if !strings.Contains(content, "hooks = true") && !strings.Contains(content, "codex_hooks = true") {
				out.WriteString("hooks = true\n")
			}
		}
	}
	if !foundFeatures {
		if content != "" && !strings.HasSuffix(content, "\n") {
			out.WriteString("\n")
		}
		out.WriteString("[features]\nhooks = true\n")
	}
	return os.WriteFile(configPath, []byte(out.String()), 0o644)
}

func dirOf(path string) string {
	for i := len(path) - 1; i >= 0; i-- {
		if path[i] == '/' || path[i] == '\\' {
			return path[:i]
		}
	}
	return "."
}

func equalsHook(a, b any) bool {
	aj, _ := json.Marshal(a)
	bj, _ := json.Marshal(b)
	return string(aj) == string(bj)
}

// hookCommandPresent reports whether cmd already appears as a command string
// in any hook inside the existing event entry. The entry is the value stored
// at hooks["EventName"] — a []any of hook-group objects, each with a "hooks"
// []any of individual hook maps.
func hookCommandPresent(entry any, cmd string) bool {
	groups, ok := entry.([]any)
	if !ok {
		return false
	}
	for _, rawGroup := range groups {
		group, ok := rawGroup.(map[string]any)
		if !ok {
			continue
		}
		rawHooks, ok := group["hooks"].([]any)
		if !ok {
			continue
		}
		for _, rawHook := range rawHooks {
			h, ok := rawHook.(map[string]any)
			if !ok {
				continue
			}
			if c, _ := h["command"].(string); c == cmd {
				return true
			}
		}
	}
	return false
}
