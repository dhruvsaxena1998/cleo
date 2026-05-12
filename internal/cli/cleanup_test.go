package cli

import (
	"bufio"
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

func TestPromptCleanupSelection(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantKeys []string
	}{
		{
			name:     "all defaults (enter×2)",
			input:    "\n\n",
			wantKeys: []string{hookClaude, hookCodex},
		},
		{
			name:     "claude only",
			input:    "y\nn\n",
			wantKeys: []string{hookClaude},
		},
		{
			name:     "none selected",
			input:    "n\nn\n",
			wantKeys: []string{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			br := bufio.NewReader(strings.NewReader(tt.input))
			var w bytes.Buffer
			var selected []string
			if err := promptCleanupSelection(&w, br, &selected); err != nil {
				t.Fatal(err)
			}
			if len(selected) != len(tt.wantKeys) {
				t.Fatalf("got %v, want %v", selected, tt.wantKeys)
			}
			for i, k := range tt.wantKeys {
				if selected[i] != k {
					t.Errorf("index %d: got %q, want %q", i, selected[i], k)
				}
			}
		})
	}
}
