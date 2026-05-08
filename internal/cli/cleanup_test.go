package cli

import (
	"bytes"
	"strings"
	"testing"
)

func TestPrintCleanupSummary(t *testing.T) {
	var out bytes.Buffer

	printCleanupSummary(&out, []cleanupResult{
		{
			Name:    "Claude Code",
			Path:    "/home/me/.claude/settings.json",
			Removed: 8,
		},
		{
			Name:    "Codex",
			Path:    "/home/me/.codex/hooks.json",
			Removed: 6,
			Notes: []string{
				"left ~/.codex/config.toml [features].hooks unchanged; that flag may be used by other Codex hooks",
			},
		},
	})

	got := out.String()
	for _, want := range []string{
		"Cleo hook cleanup complete",
		"Claude Code",
		"removed: 8 Cleo hook command(s)",
		"Codex",
		"removed: 6 Cleo hook command(s)",
		"[features].hooks unchanged",
		"Run cleo init again",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("output missing %q:\n%s", want, got)
		}
	}
}
