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
	if err := InstallClaude(settingsPath, "/usr/local/bin/cleo"); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(settingsPath)
	var got map[string]any
	_ = json.Unmarshal(b, &got)
	hooks, ok := got["hooks"].(map[string]any)
	if !ok {
		t.Fatal("hooks key missing")
	}
	for _, ev := range []string{"PreToolUse", "PostToolUse", "Notification", "Stop", "SessionStart", "SessionEnd"} {
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

	err := InstallClaude(settingsPath, "/cleo")
	if err == nil || !strings.Contains(err.Error(), "conflict") {
		t.Errorf("expected conflict error, got %v", err)
	}
}
