package cli

import (
	"bufio"
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

func TestPromptYN_YesInput(t *testing.T) {
	br := bufio.NewReader(strings.NewReader("y\n"))
	var w bytes.Buffer
	got, err := promptYN(&w, br, "Some option", true)
	if err != nil {
		t.Fatal(err)
	}
	if !got {
		t.Error("expected true for 'y' input")
	}
}

func TestPromptYN_NoInput(t *testing.T) {
	br := bufio.NewReader(strings.NewReader("n\n"))
	var w bytes.Buffer
	got, err := promptYN(&w, br, "Some option", true)
	if err != nil {
		t.Fatal(err)
	}
	if got {
		t.Error("expected false for 'n' input")
	}
}

func TestPromptYN_BlankDefaultYes(t *testing.T) {
	br := bufio.NewReader(strings.NewReader("\n"))
	var w bytes.Buffer
	got, err := promptYN(&w, br, "Some option", true)
	if err != nil {
		t.Fatal(err)
	}
	if !got {
		t.Error("expected true for blank input when defaultYes=true")
	}
}

func TestPromptYN_BlankDefaultNo(t *testing.T) {
	br := bufio.NewReader(strings.NewReader("\n"))
	var w bytes.Buffer
	got, err := promptYN(&w, br, "Some option", false)
	if err != nil {
		t.Fatal(err)
	}
	if got {
		t.Error("expected false for blank input when defaultYes=false")
	}
}

func TestPromptYN_UppercaseYN(t *testing.T) {
	for _, input := range []string{"Y\n", "N\n"} {
		br := bufio.NewReader(strings.NewReader(input))
		var w bytes.Buffer
		got, err := promptYN(&w, br, "opt", true)
		if err != nil {
			t.Fatal(err)
		}
		want := strings.ToLower(strings.TrimSpace(input)) == "y"
		if got != want {
			t.Errorf("input %q: expected %v", input, want)
		}
	}
}

func TestPromptYN_PrintsBracket(t *testing.T) {
	br := bufio.NewReader(strings.NewReader("\n"))
	var w bytes.Buffer
	if _, err := promptYN(&w, br, "My option", true); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(w.String(), "[Y/n]") {
		t.Errorf("expected [Y/n] in output, got: %s", w.String())
	}

	var w2 bytes.Buffer
	br2 := bufio.NewReader(strings.NewReader("\n"))
	if _, err := promptYN(&w2, br2, "My option", false); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(w2.String(), "[y/N]") {
		t.Errorf("expected [y/N] in output, got: %s", w2.String())
	}
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

func TestPromptHookSelection(t *testing.T) {
	tests := []struct {
		name     string
		input    string // one line per agent: claude, codex, pi, opencode
		wantKeys []string
	}{
		{
			name:     "all defaults (enter×4)",
			input:    "\n\n\n\n",
			wantKeys: []string{hookClaude, hookCodex}, // pi and opencode default to no
		},
		{
			name:     "select all",
			input:    "y\ny\ny\ny\n",
			wantKeys: []string{hookClaude, hookCodex, hookPi, hookOpenCode},
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
			if err := promptHookSelection(&w, br, &selected); err != nil {
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

func TestPrintInitSummary_OpenCode(t *testing.T) {
	var buf bytes.Buffer
	printInitSummary(&buf, []initInstallResult{
		{
			Name:           "OpenCode",
			Files:          []string{"plugin: /home/user/.config/opencode/plugins/cleo.ts"},
			InstalledHooks: []string{"session.created", "tool.execute.before", "tool.execute.after", "permission.asked", "session.idle", "session.deleted", "session.error"},
		},
	})
	out := stripANSI(buf.String())

	for _, want := range []string{
		"Cleo hooks initialized",
		"OpenCode",
		"/home/user/.config/opencode/plugins/cleo.ts",
		"7 events",
		"session.created",
		"session.idle",
		"session.deleted",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q\ngot:\n%s", want, out)
		}
	}
	if strings.Contains(out, "manual hook approval") {
		t.Errorf("opencode should not show the Codex approval step:\n%s", out)
	}
}
