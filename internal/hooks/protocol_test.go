package hooks

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestProtocolDisplayNames(t *testing.T) {
	want := map[string]string{
		"claude":   "Claude Code",
		"codex":    "Codex",
		"pi":       "Pi",
		"opencode": "OpenCode",
	}
	for _, p := range Protocols() {
		if got := p.DisplayName(); got != want[p.Name()] {
			t.Errorf("%s DisplayName() = %q, want %q", p.Name(), got, want[p.Name()])
		}
	}
}

func TestProtocolLocationsRootedAtHome(t *testing.T) {
	dir := t.TempDir()
	setTestHome(t, dir)

	want := map[string][]string{
		"claude":   {filepath.Join(dir, ".claude", "settings.json")},
		"codex":    {filepath.Join(dir, ".codex", "hooks.json"), filepath.Join(dir, ".codex", "config.toml")},
		"pi":       {filepath.Join(dir, ".pi", "agent", "extensions", "cleo.ts")},
		"opencode": {filepath.Join(dir, ".config", "opencode", "plugins", "cleo.ts")},
	}
	for _, p := range Protocols() {
		locs := p.Locations()
		var paths []string
		for _, l := range locs {
			if l.Label == "" {
				t.Errorf("%s: location has empty label: %+v", p.Name(), l)
			}
			paths = append(paths, l.Path)
		}
		got := strings.Join(paths, ",")
		exp := strings.Join(want[p.Name()], ",")
		if got != exp {
			t.Errorf("%s Locations() paths = %q, want %q", p.Name(), got, exp)
		}
	}
}

func TestInstallReportsCodexManualReview(t *testing.T) {
	dir := t.TempDir()
	setTestHome(t, dir)

	for _, p := range Protocols() {
		report, err := p.Install("/usr/local/bin/cleo", false)
		if err != nil {
			t.Fatalf("%s Install: %v", p.Name(), err)
		}
		if p.Name() == "codex" {
			if report.ManualReview == nil {
				t.Errorf("codex Install must report a manual-review step")
				continue
			}
			if !strings.Contains(report.ManualReview.Command, "hooks invoke codex") {
				t.Errorf("codex review command = %q, want it to mention 'hooks invoke codex'", report.ManualReview.Command)
			}
			if len(report.ManualReview.Hooks) == 0 {
				t.Error("codex review step must list the hooks awaiting approval")
			}
		} else if report.ManualReview != nil {
			t.Errorf("%s Install must not require manual review, got %+v", p.Name(), report.ManualReview)
		}
	}
}

func TestDiagnoseReportsHealthyAfterInstall(t *testing.T) {
	dir := t.TempDir()
	setTestHome(t, dir)

	for _, p := range Protocols() {
		if _, err := p.Install("/usr/local/bin/cleo", false); err != nil {
			t.Fatalf("%s Install: %v", p.Name(), err)
		}
		for _, c := range p.Diagnose() {
			if !c.OK {
				t.Errorf("%s Diagnose after install not OK: %q — %s", p.Name(), c.Label, c.Detail)
			}
		}
	}
}

func TestDiagnoseReportsMissingConfig(t *testing.T) {
	dir := t.TempDir()
	setTestHome(t, dir)

	for _, p := range Protocols() {
		checks := p.Diagnose()
		if len(checks) == 0 {
			t.Errorf("%s Diagnose returned no checks", p.Name())
		}
		for _, c := range checks {
			if c.OK {
				t.Errorf("%s Diagnose on empty home should fail: %q OK=true", p.Name(), c.Label)
			}
		}
	}
}

func TestCodexCleanupCarriesConfigNote(t *testing.T) {
	dir := t.TempDir()
	setTestHome(t, dir)
	if _, err := (CodexProtocol{}).Install("/usr/local/bin/cleo", false); err != nil {
		t.Fatal(err)
	}
	outcome, err := (CodexProtocol{}).Cleanup()
	if err != nil {
		t.Fatal(err)
	}
	if len(outcome.Notes) == 0 {
		t.Fatal("codex Cleanup must carry a note about the leftover config.toml flag")
	}
	if !strings.Contains(strings.Join(outcome.Notes, " "), "config.toml") {
		t.Errorf("codex cleanup note should mention config.toml, got %v", outcome.Notes)
	}
}
