package cli

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dhruvsaxena1998/cleo/internal/hooks"
)

func TestDiagnoseHooksReportsHealthySetup(t *testing.T) {
	dir := t.TempDir()
	claudePath := filepath.Join(dir, ".claude", "settings.json")
	codexHooksPath := filepath.Join(dir, ".codex", "hooks.json")
	codexConfigPath := filepath.Join(dir, ".codex", "config.toml")
	tracePath := filepath.Join(dir, "hook-trace.log")

	if err := os.MkdirAll(filepath.Dir(claudePath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := hooks.InstallClaude(claudePath, "/usr/local/bin/cleo", false); err != nil {
		t.Fatal(err)
	}
	if err := hooks.InstallCodex(codexHooksPath, codexConfigPath, "/usr/local/bin/cleo", false); err != nil {
		t.Fatal(err)
	}

	trace := `{"at":"now","protocol":"claude","event":"Stop","env_session":true,"resolved_session":"cleo-x-claude-1","result":"resolved"}` + "\n" +
		`{"at":"now","protocol":"codex","event":"Stop","env_session":true,"resolved_session":"cleo-x-codex-1","result":"resolved"}` + "\n"
	if err := os.WriteFile(tracePath, []byte(trace), 0o644); err != nil {
		t.Fatal(err)
	}

	report := diagnoseHooks(claudePath, codexHooksPath, codexConfigPath, tracePath)
	got := fmt.Sprint(report.Checks)
	for _, want := range []string{"Claude hook activity", "Codex hook activity"} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected %q in diagnose checks, got %+v", want, report.Checks)
		}
	}
	for _, check := range report.Checks {
		if !check.OK {
			t.Fatalf("expected healthy check, got %+v", check)
		}
	}
}

func TestDiagnoseHooksReportsMissingCodexHook(t *testing.T) {
	dir := t.TempDir()
	claudePath := filepath.Join(dir, ".claude", "settings.json")
	codexHooksPath := filepath.Join(dir, ".codex", "hooks.json")
	codexConfigPath := filepath.Join(dir, ".codex", "config.toml")
	tracePath := filepath.Join(dir, "hook-trace.log")

	if err := os.MkdirAll(filepath.Dir(claudePath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := hooks.InstallClaude(claudePath, "/usr/local/bin/cleo", false); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(codexHooksPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(codexConfigPath, []byte("[features]\nhooks = true\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(codexHooksPath, []byte(`{"hooks":{"SessionStart":[{"hooks":[{"command":"/usr/local/bin/cleo hook codex SessionStart"}]}]}}`), 0o644); err != nil {
		t.Fatal(err)
	}

	report := diagnoseHooks(claudePath, codexHooksPath, codexConfigPath, tracePath)
	got := fmt.Sprint(report.Checks)
	if !strings.Contains(got, "PreToolUse") {
		t.Fatalf("expected missing codex hook detail, got %+v", report.Checks)
	}
}

func TestPrintDoctorReportListsCodexApprovalHooks(t *testing.T) {
	var out bytes.Buffer

	printDoctorReport(&out, doctorReport{Checks: []doctorCheck{
		{Label: "Codex hooks", OK: true, Detail: "6 hooks installed"},
	}})

	got := out.String()
	for _, want := range []string{
		"Cleo doctor",
		"[ok] Codex hooks",
		"Codex approval check",
		"SessionStart",
		"UserPromptSubmit",
		"PermissionRequest",
		"run /hooks",
		"Do not run hook commands manually",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("output missing %q:\n%s", want, got)
		}
	}
}
