package hooks

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
)

var claudeEvents = []string{"PreToolUse", "PostToolUse", "Notification", "Stop", "SessionStart", "SessionEnd", "SubagentStop"}

func InstallClaude(settingsPath, cleoBin string) error {
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
	for _, ev := range claudeEvents {
		want := []any{
			map[string]any{
				"hooks": []any{
					map[string]any{
						"type":    "command",
						"command": fmt.Sprintf("%s hook claude %s", cleoBin, ev),
						"timeout": 2,
					},
				},
			},
		}
		if existing, ok := hooks[ev]; ok {
			if !equalsHook(existing, want) {
				return fmt.Errorf("conflict: %s already has a different hook (re-run with --force to overwrite)", ev)
			}
		}
		hooks[ev] = want
	}
	settings["hooks"] = hooks
	out, _ := json.MarshalIndent(settings, "", "  ")
	return os.WriteFile(settingsPath, out, 0o644)
}

func equalsHook(a, b any) bool {
	aj, _ := json.Marshal(a)
	bj, _ := json.Marshal(b)
	return string(aj) == string(bj)
}
