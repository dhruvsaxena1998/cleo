package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

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

func TestDoctorPrintsRecentTraces(t *testing.T) {
	dir := t.TempDir()
	tracePath := filepath.Join(dir, "hook-trace.log")
	rows := []string{
		`{"at":"2026-05-01T12:00:00Z","protocol":"claude","event":"SessionStart","resolved_session":"sid-a","result":"resolved","fallback_reason":"env_present"}`,
		`{"at":"2026-05-01T12:00:01Z","protocol":"claude","event":"PreToolUse","resolved_session":"sid-a","result":"resolved","fallback_reason":"env_present"}`,
		`{"at":"2026-05-01T12:00:02Z","protocol":"claude","event":"Stop","resolved_session":"sid-a","result":"resolved","fallback_reason":"env_present"}`,
		`{"at":"2026-05-01T12:00:03Z","protocol":"claude","event":"Notification","resolved_session":"sid-a","result":"resolved","fallback_reason":"env_present"}`,
	}
	if err := os.WriteFile(tracePath, []byte(strings.Join(rows, "\n")+"\n"), 0o644); err != nil {
		t.Fatalf("write trace: %v", err)
	}

	got := recentHookTraces(tracePath, "claude", 3)
	if len(got) != 3 {
		t.Fatalf("len: want 3, got %d", len(got))
	}
	if got[0].Event != "Notification" { // most recent first
		t.Errorf("ordering: want Notification first, got %s", got[0].Event)
	}
}

func TestDoctorAttributionFailureSummary(t *testing.T) {
	dir := t.TempDir()
	tracePath := filepath.Join(dir, "hook-trace.log")
	rows := []string{
		`{"at":"2026-05-01T12:00:00Z","protocol":"codex","event":"PreToolUse","cwd":"/a","result":"resolved","fallback_reason":"env_missing"}`,
		`{"at":"2026-05-01T12:00:01Z","protocol":"codex","event":"PreToolUse","cwd":"/a","result":"ignored:no_session","fallback_reason":"no_match"}`,
		`{"at":"2026-05-01T12:00:02Z","protocol":"claude","event":"PreToolUse","result":"ignored:no_session","fallback_reason":"env_unknown_session"}`,
	}
	if err := os.WriteFile(tracePath, []byte(strings.Join(rows, "\n")+"\n"), 0o644); err != nil {
		t.Fatalf("write trace: %v", err)
	}

	failures := attributionFailures(tracePath, time.Time{})
	if len(failures) != 2 {
		t.Fatalf("len: want 2, got %d (%+v)", len(failures), failures)
	}
}

func TestDoctorHookConfigDiff(t *testing.T) {
	dir := t.TempDir()
	settings := filepath.Join(dir, "settings.json")
	// On-disk: SessionStart matches, PreToolUse exists but has the wrong
	// command (a foreign or stale entry — should land in conflicts), no
	// UserPromptSubmit at all (should land in toAdd).
	onDisk := map[string]any{
		"hooks": map[string]any{
			"SessionStart": map[string]any{"command": "/path/to/cleo hook claude SessionStart"},
			"PreToolUse":   map[string]any{"command": "/usr/local/bin/some-other-hook"},
		},
	}
	b, _ := json.Marshal(onDisk)
	if err := os.WriteFile(settings, b, 0o644); err != nil {
		t.Fatalf("seed settings: %v", err)
	}

	expectedEntries := map[string]any{
		"SessionStart":     map[string]any{"command": "/path/to/cleo hook claude SessionStart"},
		"UserPromptSubmit": map[string]any{"command": "/path/to/cleo hook claude UserPromptSubmit"},
		"PreToolUse":       map[string]any{"command": "/path/to/cleo hook claude PreToolUse"},
	}

	d := hookConfigDiff(settings, expectedEntries)
	if !contains(d.matched, "SessionStart") {
		t.Errorf("matched should include SessionStart: %+v", d)
	}
	if !contains(d.toAdd, "UserPromptSubmit") {
		t.Errorf("toAdd should include UserPromptSubmit: %+v", d)
	}
	if !contains(d.conflicts, "PreToolUse") {
		t.Errorf("conflicts should include PreToolUse (foreign command on disk): %+v", d)
	}
}

// JSON marshal output for map[string]any has been deterministic since Go 1.12
// (sorted keys), which the diff's string-equality check relies on. Lock that
// assumption in: same logical content, two different programmatic build
// orders, must compare equal.
func TestHookConfigDiffEqualityIsKeyOrderInsensitive(t *testing.T) {
	dir := t.TempDir()
	settings := filepath.Join(dir, "settings.json")
	// Construct on-disk JSON with a hand-written byte order that puts "type"
	// after "command" — different from Go's marshal-sorted order.
	raw := []byte(`{"hooks":{"SessionStart":{"timeout":2,"command":"cleo hook claude SessionStart","type":"command"}}}`)
	if err := os.WriteFile(settings, raw, 0o644); err != nil {
		t.Fatalf("seed: %v", err)
	}
	expected := map[string]any{
		"SessionStart": map[string]any{
			"command": "cleo hook claude SessionStart",
			"type":    "command",
			"timeout": float64(2), // JSON unmarshal produces float64 for numbers
		},
	}
	d := hookConfigDiff(settings, expected)
	if !contains(d.matched, "SessionStart") {
		t.Errorf("expected matched (key-order shouldn't affect equality): %+v", d)
	}
	if len(d.conflicts) != 0 {
		t.Errorf("expected no conflicts, got %+v", d.conflicts)
	}
}

func contains(xs []string, s string) bool {
	for _, x := range xs {
		if x == s {
			return true
		}
	}
	return false
}

func TestDoctorQuietSuppressesPassingChecks(t *testing.T) {
	report := doctorReport{
		Checks: []doctorCheck{
			{Label: "Claude hooks", OK: true, Detail: "8 hooks installed"},
			{Label: "Codex feature flag", OK: false, Detail: "missing"},
		},
	}
	var buf bytes.Buffer
	printDoctorReportOpts(&buf, report, analyzeReport(report), doctorPrintOpts{Quiet: true})
	out := buf.String()
	if strings.Contains(out, "Claude hooks") {
		t.Errorf("quiet mode should hide passing check, got %q", out)
	}
	if !strings.Contains(out, "Codex feature flag") {
		t.Errorf("quiet mode should still show failure, got %q", out)
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
		"✓",
		"Codex hooks",
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
