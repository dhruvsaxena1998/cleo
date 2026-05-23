package hooks

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInstallClaudeHooks(t *testing.T) {
	dir := t.TempDir()
	settingsPath := filepath.Join(dir, "settings.json")
	if err := os.WriteFile(settingsPath, []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := InstallClaude(settingsPath, "/usr/local/bin/cleo", false); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(settingsPath)
	var got map[string]any
	_ = json.Unmarshal(b, &got)
	hooks, ok := got["hooks"].(map[string]any)
	if !ok {
		t.Fatal("hooks key missing")
	}
	for _, ev := range []string{"SessionStart", "UserPromptSubmit", "PreToolUse", "PostToolUse", "Notification", "Stop", "SessionEnd", "SubagentStop"} {
		if hooks[ev] == nil {
			t.Errorf("missing %s", ev)
		}
	}
	// And the path is absolute
	if !strings.Contains(string(b), "/usr/local/bin/cleo hook claude") {
		t.Errorf("hook command not present: %s", string(b))
	}
}

func TestInstallClaudeRefusesPreExistingDifferentValue(t *testing.T) {
	dir := t.TempDir()
	settingsPath := filepath.Join(dir, "settings.json")
	prior := `{"hooks":{"PreToolUse":[{"hooks":[{"command":"some-other-tool"}]}]}}`
	_ = os.WriteFile(settingsPath, []byte(prior), 0o644)

	err := InstallClaude(settingsPath, "/cleo", false)
	if err == nil || !strings.Contains(err.Error(), "conflict") {
		t.Errorf("expected conflict error, got %v", err)
	}
}

func TestInstallClaudeForceOverwrites(t *testing.T) {
	dir := t.TempDir()
	settingsPath := filepath.Join(dir, "settings.json")
	prior := `{"hooks":{"PreToolUse":[{"hooks":[{"command":"some-other-tool"}]}]}}`
	_ = os.WriteFile(settingsPath, []byte(prior), 0o644)

	if err := InstallClaude(settingsPath, "/cleo", true); err != nil {
		t.Fatalf("force install failed: %v", err)
	}
	b, _ := os.ReadFile(settingsPath)
	if !strings.Contains(string(b), "/cleo hook claude") {
		t.Errorf("hook command not overwritten: %s", string(b))
	}
}

func TestCleanupClaudeRemovesOnlyCleoHooks(t *testing.T) {
	dir := t.TempDir()
	settingsPath := filepath.Join(dir, "settings.json")
	prior := `{
  "hooks": {
    "PreToolUse": [
      {
        "hooks": [
          {"type":"command","command":"/usr/local/bin/cleo hook claude PreToolUse","timeout":2},
          {"type":"command","command":"other-tool pre"}
        ]
      }
    ],
    "Stop": [
      {
        "hooks": [
          {"type":"command","command":"/old/path/cleo hook claude Stop","timeout":2}
        ]
      }
    ]
  },
  "theme": "dark"
}`
	_ = os.WriteFile(settingsPath, []byte(prior), 0o644)

	outcome, err := CleanupClaude(settingsPath)
	if err != nil {
		t.Fatal(err)
	}
	if outcome.Status != CleanupStatusRemoved {
		t.Fatalf("Status = %v, want CleanupStatusRemoved", outcome.Status)
	}
	if outcome.Path != settingsPath {
		t.Errorf("Path = %q, want %q", outcome.Path, settingsPath)
	}

	b, _ := os.ReadFile(settingsPath)
	got := string(b)
	if strings.Contains(got, "hook claude") {
		t.Fatalf("cleo hook still present: %s", got)
	}
	if !strings.Contains(got, "other-tool pre") {
		t.Fatalf("unrelated hook was removed: %s", got)
	}
	if strings.Contains(got, `"Stop"`) {
		t.Fatalf("empty event was not removed: %s", got)
	}
	if !strings.Contains(got, `"theme": "dark"`) {
		t.Fatalf("unrelated setting was removed: %s", got)
	}
}

func TestCleanupClaude_MissingWhenFileAbsent(t *testing.T) {
	dir := t.TempDir()
	settingsPath := filepath.Join(dir, "does-not-exist.json")

	outcome, err := CleanupClaude(settingsPath)
	if err != nil {
		t.Fatal(err)
	}
	if outcome.Status != CleanupStatusMissing {
		t.Errorf("Status = %v, want CleanupStatusMissing", outcome.Status)
	}
}

func TestCleanupClaude_MissingWhenNoCleoEntries(t *testing.T) {
	dir := t.TempDir()
	settingsPath := filepath.Join(dir, "settings.json")
	// Pre-existing settings with a non-cleo hook only.
	prior := `{"hooks":{"PreToolUse":[{"hooks":[{"type":"command","command":"other-tool"}]}]}}`
	_ = os.WriteFile(settingsPath, []byte(prior), 0o644)

	outcome, err := CleanupClaude(settingsPath)
	if err != nil {
		t.Fatal(err)
	}
	if outcome.Status != CleanupStatusMissing {
		t.Errorf("Status = %v, want CleanupStatusMissing (no cleo entries to remove)", outcome.Status)
	}
	// Unrelated hook must still be on disk.
	b, _ := os.ReadFile(settingsPath)
	if !strings.Contains(string(b), "other-tool") {
		t.Errorf("unrelated hook was disturbed: %s", string(b))
	}
}

func TestInstallCodexHooks(t *testing.T) {
	dir := t.TempDir()
	hooksPath := filepath.Join(dir, "hooks.json")
	configPath := filepath.Join(dir, "config.toml")

	if err := InstallCodex(hooksPath, configPath, "/usr/local/bin/cleo", false); err != nil {
		t.Fatal(err)
	}

	// Verify hooks.json has all expected events
	b, _ := os.ReadFile(hooksPath)
	var got map[string]any
	_ = json.Unmarshal(b, &got)
	hooks, ok := got["hooks"].(map[string]any)
	if !ok {
		t.Fatal("hooks key missing in hooks.json")
	}
	for _, ev := range []string{"SessionStart", "UserPromptSubmit", "PreToolUse", "PostToolUse", "PermissionRequest", "Stop"} {
		if hooks[ev] == nil {
			t.Errorf("missing event %s", ev)
		}
	}
	if !strings.Contains(string(b), "/usr/local/bin/cleo hook codex") {
		t.Errorf("hook command not present: %s", string(b))
	}

	// Verify config.toml has the feature flag
	cfg, _ := os.ReadFile(configPath)
	if !strings.Contains(string(cfg), "hooks = true") {
		t.Errorf("feature flag missing in config.toml: %s", string(cfg))
	}
}

func TestCleanupCodexRemovesOnlyCleoHooks(t *testing.T) {
	dir := t.TempDir()
	hooksPath := filepath.Join(dir, "hooks.json")
	if err := InstallCodex(hooksPath, filepath.Join(dir, "config.toml"), "/usr/local/bin/cleo", false); err != nil {
		t.Fatal(err)
	}

	outcome, err := CleanupCodex(hooksPath)
	if err != nil {
		t.Fatal(err)
	}
	if outcome.Status != CleanupStatusRemoved {
		t.Fatalf("Status = %v, want CleanupStatusRemoved", outcome.Status)
	}
	if outcome.Path != hooksPath {
		t.Errorf("Path = %q, want %q", outcome.Path, hooksPath)
	}

	b, _ := os.ReadFile(hooksPath)
	got := string(b)
	if strings.Contains(got, "hook codex") {
		t.Fatalf("cleo codex hook still present: %s", got)
	}
	if strings.Contains(got, `"hooks"`) {
		t.Fatalf("empty hooks section was not removed: %s", got)
	}
}

func TestInstallCodexFeatureFlagIdempotent(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.toml")
	_ = os.WriteFile(configPath, []byte("[features]\nhooks = true\n"), 0o644)

	if err := ensureCodexFeatureFlag(configPath); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(configPath)
	count := strings.Count(string(b), "hooks")
	if count != 1 {
		t.Errorf("expected 1 occurrence of hooks, got %d: %s", count, string(b))
	}
}

func TestInstallCodexFeatureFlagMigratesDeprecatedFlag(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.toml")
	_ = os.WriteFile(configPath, []byte("[features]\ncodex_hooks = true\nterminal_resize_reflow = true\n"), 0o644)

	if err := ensureCodexFeatureFlag(configPath); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(configPath)
	if strings.Contains(string(b), "codex_hooks") {
		t.Errorf("deprecated flag not removed: %s", string(b))
	}
	if !strings.Contains(string(b), "hooks = true") {
		t.Errorf("feature flag not migrated: %s", string(b))
	}
}

func TestInstallCodexFeatureFlagAppendsToExistingFeatures(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.toml")
	_ = os.WriteFile(configPath, []byte("[features]\nsome_other = true\n"), 0o644)

	if err := ensureCodexFeatureFlag(configPath); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(configPath)
	if !strings.Contains(string(b), "hooks = true") {
		t.Errorf("feature flag not added: %s", string(b))
	}
	if strings.Count(string(b), "[features]") != 1 {
		t.Errorf("duplicate [features] section: %s", string(b))
	}
}

func TestInstallCodexConflictRefused(t *testing.T) {
	dir := t.TempDir()
	hooksPath := filepath.Join(dir, "hooks.json")
	configPath := filepath.Join(dir, "config.toml")
	prior := `{"hooks":{"SessionStart":[{"hooks":[{"command":"other-tool"}]}]}}`
	_ = os.WriteFile(hooksPath, []byte(prior), 0o644)

	err := InstallCodex(hooksPath, configPath, "/cleo", false)
	if err == nil || !strings.Contains(err.Error(), "conflict") {
		t.Errorf("expected conflict error, got %v", err)
	}
}

func TestInstallClaudeIdempotent(t *testing.T) {
	dir := t.TempDir()
	settingsPath := filepath.Join(dir, "settings.json")
	_ = os.WriteFile(settingsPath, []byte("{}"), 0o644)

	// First install
	if err := InstallClaude(settingsPath, "/usr/local/bin/cleo", false); err != nil {
		t.Fatalf("first install: %v", err)
	}
	b1, _ := os.ReadFile(settingsPath)

	// Second install — must not error and must produce identical output
	if err := InstallClaude(settingsPath, "/usr/local/bin/cleo", false); err != nil {
		t.Fatalf("second install: %v", err)
	}
	b2, _ := os.ReadFile(settingsPath)

	if string(b1) != string(b2) {
		t.Errorf("second install mutated settings.json:\nbefore: %s\nafter:  %s", b1, b2)
	}
}

func TestInstallCodexIdempotent(t *testing.T) {
	dir := t.TempDir()
	hooksPath := filepath.Join(dir, "hooks.json")
	configPath := filepath.Join(dir, "config.toml")

	if err := InstallCodex(hooksPath, configPath, "/usr/local/bin/cleo", false); err != nil {
		t.Fatalf("first install: %v", err)
	}
	b1, _ := os.ReadFile(hooksPath)

	if err := InstallCodex(hooksPath, configPath, "/usr/local/bin/cleo", false); err != nil {
		t.Fatalf("second install: %v", err)
	}
	b2, _ := os.ReadFile(hooksPath)

	if string(b1) != string(b2) {
		t.Errorf("second install mutated hooks.json:\nbefore: %s\nafter:  %s", b1, b2)
	}
}

func TestClaudeHookTimeoutIs5Seconds(t *testing.T) {
	entries := ExpectedClaudeEntries("/usr/local/bin/cleo")
	for ev, rawEntry := range entries {
		entryList, ok := rawEntry.([]any)
		if !ok || len(entryList) == 0 {
			t.Fatalf("event %s: unexpected shape", ev)
		}
		entry, ok := entryList[0].(map[string]any)
		if !ok {
			t.Fatalf("event %s: entry not a map", ev)
		}
		hooks, ok := entry["hooks"].([]any)
		if !ok || len(hooks) == 0 {
			t.Fatalf("event %s: no hooks", ev)
		}
		hook, ok := hooks[0].(map[string]any)
		if !ok {
			t.Fatalf("event %s: hook not a map", ev)
		}
		if timeout, _ := hook["timeout"].(int); timeout != 5 {
			t.Errorf("event %s: want timeout 5, got %v", ev, hook["timeout"])
		}
	}
}

func TestCodexHookTimeoutIs5Seconds(t *testing.T) {
	entries := ExpectedCodexEntries("/usr/local/bin/cleo")
	for ev, rawEntry := range entries {
		entryList, ok := rawEntry.([]any)
		if !ok || len(entryList) == 0 {
			t.Fatalf("event %s: unexpected shape", ev)
		}
		entry, ok := entryList[0].(map[string]any)
		if !ok {
			t.Fatalf("event %s: entry not a map", ev)
		}
		hooks, ok := entry["hooks"].([]any)
		if !ok || len(hooks) == 0 {
			t.Fatalf("event %s: no hooks", ev)
		}
		hook, ok := hooks[0].(map[string]any)
		if !ok {
			t.Fatalf("event %s: hook not a map", ev)
		}
		if timeout, _ := hook["timeout"].(int); timeout != 5 {
			t.Errorf("event %s: want timeout 5, got %v", ev, hook["timeout"])
		}
	}
}
