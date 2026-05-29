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

// expectedJSONEntries builds the per-event hook entries cleo writes for a
// JSON-hook agent, keyed by event name. protocol is the agent name embedded in
// the invoke command. The shape matches what Claude Code / Codex expect on disk.
func expectedJSONEntries(protocol, cleoBin string, events []string) map[string]any {
	out := make(map[string]any, len(events))
	for _, ev := range events {
		out[ev] = []any{
			map[string]any{
				"hooks": []any{
					map[string]any{
						"type":    "command",
						"command": fmt.Sprintf("%s hooks invoke %s %s", cleoBin, protocol, ev),
						"timeout": 5,
					},
				},
			},
		}
	}
	return out
}

// installJSONHooks writes cleo hook entries for each event into the JSON config
// at path (creating the parent directory). It skips events already wired to
// cleo, refuses to clobber a foreign entry unless force, and is idempotent.
// protocol names the agent in the invoke command; fileLabel names the file in
// parse-error messages.
func installJSONHooks(path, protocol, fileLabel, cleoBin string, events []string, force bool) error {
	if err := os.MkdirAll(dirOf(path), 0o755); err != nil {
		return err
	}
	b, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		b = []byte("{}")
	} else if err != nil {
		return err
	}
	var settings map[string]any
	if err := json.Unmarshal(b, &settings); err != nil {
		return fmt.Errorf("%s: %w", fileLabel, err)
	}
	hooks, _ := settings["hooks"].(map[string]any)
	if hooks == nil {
		hooks = map[string]any{}
	}
	expected := expectedJSONEntries(protocol, cleoBin, events)
	for _, ev := range events {
		want := expected[ev]
		cmd := fmt.Sprintf("%s hooks invoke %s %s", cleoBin, protocol, ev)
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
	return os.WriteFile(path, out, 0o644)
}

// ExpectedClaudeEntries returns the per-event hook entries that InstallClaude
// would write for the given cleo binary path. Used by doctor's config diff.
func ExpectedClaudeEntries(cleoBin string) map[string]any {
	return expectedJSONEntries("claude", cleoBin, claudeEvents)
}

func InstallClaude(settingsPath, cleoBin string, force bool) error {
	return installJSONHooks(settingsPath, "claude", "settings.json", cleoBin, claudeEvents, force)
}

func CleanupClaude(settingsPath string) (CleanupOutcome, error) {
	return cleanupHookFile(settingsPath, "claude", "settings.json")
}

// diagnoseJSONHooks checks that the JSON hook-config file at path wires a cleo
// command for every expected event. label names the check; commandNeedle is the
// per-protocol command fragment ("hooks invoke claude"). Shared by the claude
// and codex Diagnose() methods.
func diagnoseJSONHooks(label, path string, events []string, commandNeedle string) Check {
	b, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return Check{Label: label, Detail: fmt.Sprintf("missing %s; run cleo hooks init", path)}
	}
	if err != nil {
		return Check{Label: label, Detail: err.Error()}
	}
	var settings map[string]any
	if err := json.Unmarshal(b, &settings); err != nil {
		return Check{Label: label, Detail: fmt.Sprintf("invalid JSON in %s: %v", path, err)}
	}
	configured, _ := settings["hooks"].(map[string]any)
	var missing []string
	for _, ev := range events {
		entry, ok := configured[ev]
		if !ok || !jsonHookEntryHasCommand(entry, commandNeedle, ev) {
			missing = append(missing, ev)
		}
	}
	if len(missing) > 0 {
		return Check{Label: label, Detail: fmt.Sprintf("missing Cleo command for %s in %s; run cleo hooks init", strings.Join(missing, ", "), path)}
	}
	return Check{Label: label, OK: true, Detail: fmt.Sprintf("%d hooks installed", len(events))}
}

func jsonHookEntryHasCommand(entry any, commandNeedle, event string) bool {
	b, err := json.Marshal(entry)
	if err != nil {
		return false
	}
	text := string(b)
	return strings.Contains(text, commandNeedle) && strings.Contains(text, event)
}

// ExpectedCodexEntries returns the per-event hook entries that InstallCodex
// would write for the given cleo binary path. Used by doctor's config diff.
func ExpectedCodexEntries(cleoBin string) map[string]any {
	return expectedJSONEntries("codex", cleoBin, codexEvents)
}

// InstallCodex writes hook entries to hooksPath (~/.codex/hooks.json) and
// ensures the feature flag is set in configPath (~/.codex/config.toml).
func InstallCodex(hooksPath, configPath, cleoBin string, force bool) error {
	if err := installJSONHooks(hooksPath, "codex", "hooks.json", cleoBin, codexEvents, force); err != nil {
		return err
	}
	return ensureCodexFeatureFlag(configPath)
}

func CleanupCodex(hooksPath string) (CleanupOutcome, error) {
	return cleanupHookFile(hooksPath, "codex", "hooks.json")
}

func cleanupHookFile(path, protocol, label string) (CleanupOutcome, error) {
	b, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return CleanupOutcome{Status: CleanupStatusMissing, Path: path}, nil
	} else if err != nil {
		return CleanupOutcome{Path: path}, err
	}
	var settings map[string]any
	if err := json.Unmarshal(b, &settings); err != nil {
		return CleanupOutcome{Path: path}, fmt.Errorf("%s: %w", label, err)
	}
	removed := removeProtocolHooks(settings, protocol)
	if removed == 0 {
		return CleanupOutcome{Status: CleanupStatusMissing, Path: path}, nil
	}
	out, _ := json.MarshalIndent(settings, "", "  ")
	if err := os.WriteFile(path, out, 0o644); err != nil {
		return CleanupOutcome{Path: path}, err
	}
	return CleanupOutcome{Status: CleanupStatusRemoved, Path: path}, nil
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
	return strings.Contains(command, " hooks invoke "+protocol+" ")
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
