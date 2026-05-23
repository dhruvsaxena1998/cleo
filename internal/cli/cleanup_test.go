package cli

import (
	"bufio"
	"bytes"
	"strings"
	"testing"

	"github.com/dhruvsaxena1998/cleo/internal/hooks"
)

func TestPrintCleanupSummary(t *testing.T) {
	var out bytes.Buffer

	printCleanupSummary(&out, []cleanupResult{
		{
			Name: "Claude Code",
			Outcome: hooks.CleanupOutcome{
				Status: hooks.CleanupStatusRemoved,
				Path:   "/home/me/.claude/settings.json",
			},
		},
		{
			Name: "Codex",
			Outcome: hooks.CleanupOutcome{
				Status: hooks.CleanupStatusRemoved,
				Path:   "/home/me/.codex/hooks.json",
			},
			Notes: []string{
				"left ~/.codex/config.toml [features].hooks unchanged; that flag may be used by other Codex hooks",
			},
		},
	})

	got := out.String()
	for _, want := range []string{
		"Cleo hook cleanup complete",
		"Claude Code",
		"removed Cleo hook entries",
		"Codex",
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
		input    string // one line per agent: claude, codex, opencode, pi
		wantKeys []string
	}{
		{
			name:     "all defaults (enter×4)",
			input:    "\n\n\n\n",
			wantKeys: []string{hookClaude, hookCodex, hookOpenCode, hookPi},
		},
		{
			name:     "claude only",
			input:    "y\nn\nn\nn\n",
			wantKeys: []string{hookClaude},
		},
		{
			name:     "none selected",
			input:    "n\nn\nn\nn\n",
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
