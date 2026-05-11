package cli

import (
	"bytes"
	"strings"
	"testing"
)

// stripANSI removes ANSI escape sequences for plain-text assertions.
func stripANSI(s string) string {
	var out strings.Builder
	inEsc := false
	for _, r := range s {
		if r == '\x1b' {
			inEsc = true
			continue
		}
		if inEsc {
			if r == 'm' {
				inEsc = false
			}
			continue
		}
		out.WriteRune(r)
	}
	return out.String()
}

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

	got := stripANSI(out.String())
	for _, want := range []string{
		"Cleo hooks initialized",
		"Codex",
		"[features].hooks = true",
		"6 events",
		"SessionStart",
		"UserPromptSubmit",
		"PreToolUse",
		"PostToolUse",
		"PermissionRequest",
		"Stop",
		"/usr/local/bin/cleo hook codex",
		"manual hook approval",
		"run /hooks",
		"Restart any open sessions",
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

	got := stripANSI(out.String())
	for _, want := range []string{"Claude Code", "8 events", "UserPromptSubmit", "Notification", "SubagentStop"} {
		if !strings.Contains(got, want) {
			t.Fatalf("output missing %q:\n%s", want, got)
		}
	}
	if strings.Contains(got, "/hooks") {
		t.Fatalf("claude-only output should not include Codex review step:\n%s", got)
	}
}

func TestPrintInitSummary_Claude(t *testing.T) {
	var buf bytes.Buffer
	printInitSummary(&buf, []initInstallResult{
		{
			Name:           "Claude Code",
			Files:          []string{"hooks: /home/user/.claude/settings.json"},
			InstalledHooks: []string{"SessionStart", "UserPromptSubmit", "PreToolUse"},
		},
	})
	out := stripANSI(buf.String())

	wantStrings := []string{
		"Cleo hooks initialized",
		"Claude Code",
		"/home/user/.claude/settings.json",
		"3 events",
		"SessionStart",
		"UserPromptSubmit",
		"PreToolUse",
	}
	for _, want := range wantStrings {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q\ngot:\n%s", want, out)
		}
	}
}

func TestPrintInitSummary_CodexApprovalBlock(t *testing.T) {
	var buf bytes.Buffer
	printInitSummary(&buf, []initInstallResult{
		{
			Name:             "Codex",
			Files:            []string{"hooks: /home/user/.codex/hooks.json"},
			InstalledHooks:   []string{"SessionStart", "Stop"},
			NeedsCodexReview: true,
			ReviewHooks:      []string{"SessionStart", "Stop"},
			ReviewCommand:    "/usr/local/bin/cleo hook codex",
		},
	})
	out := stripANSI(buf.String())

	if !strings.Contains(out, "manual hook approval") {
		t.Errorf("output missing approval warning\ngot:\n%s", out)
	}
	if !strings.Contains(out, "run /hooks") {
		t.Errorf("output missing /hooks instruction\ngot:\n%s", out)
	}
	if !strings.Contains(out, "/usr/local/bin/cleo hook codex") {
		t.Errorf("output missing review command\ngot:\n%s", out)
	}
}
