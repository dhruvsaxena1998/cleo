package cli

import (
	"bytes"
	"strings"
	"testing"
)

func TestPrintInitSummaryIncludesCodexReviewStep(t *testing.T) {
	var out bytes.Buffer

	printInitSummary(&out, []initInstallResult{
		{
			Name: "Codex",
			Files: []string{
				"hooks: /home/me/.codex/hooks.json",
				"feature flag: /home/me/.codex/config.toml ([features].hooks = true)",
			},
			InstalledHooks:   []string{"SessionStart", "UserPromptSubmit", "PreToolUse", "PostToolUse", "PermissionRequest", "Stop"},
			NeedsCodexReview: true,
			ReviewHooks:      []string{"SessionStart", "UserPromptSubmit", "PreToolUse", "PostToolUse", "PermissionRequest", "Stop"},
			ReviewCommand:    "/usr/local/bin/cleo hook codex",
		},
	})

	got := out.String()
	for _, want := range []string{
		"Cleo hooks initialized",
		"Codex",
		"[features].hooks = true",
		"events:",
		"SessionStart",
		"UserPromptSubmit",
		"PreToolUse",
		"PostToolUse",
		"PermissionRequest",
		"Stop",
		"Match command text starting with: /usr/local/bin/cleo hook codex",
		"Do not run these hook commands manually",
		"run /hooks",
		"under Review",
		"until this review step is completed",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("output missing %q:\n%s", want, got)
		}
	}
	if strings.Contains(got, "codex_hooks") {
		t.Fatalf("output should not mention deprecated codex_hooks flag:\n%s", got)
	}
}

func TestPrintInitSummaryOmitsCodexReviewStepForClaudeOnly(t *testing.T) {
	var out bytes.Buffer

	printInitSummary(&out, []initInstallResult{
		{
			Name:           "Claude Code",
			Files:          []string{"hooks: /home/me/.claude/settings.json"},
			InstalledHooks: []string{"SessionStart", "UserPromptSubmit", "PreToolUse", "PostToolUse", "Notification", "Stop", "SessionEnd", "SubagentStop"},
		},
	})

	got := out.String()
	for _, want := range []string{"Claude Code", "UserPromptSubmit", "Notification", "SubagentStop"} {
		if !strings.Contains(got, want) {
			t.Fatalf("output missing %q:\n%s", want, got)
		}
	}
	if strings.Contains(got, "/hooks") {
		t.Fatalf("claude-only output should not include Codex review step:\n%s", got)
	}
}
