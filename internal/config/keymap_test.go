package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDefaultLoadResolvesDefaultKeymap(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")

	c, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}

	if got := c.Keymap.Up.Keys(); !equalKeys(got, []string{"up", "k"}) {
		t.Errorf("Up keys = %v, want [up k]", got)
	}
}

func TestPerActionReplaceOverridesKeysEntirely(t *testing.T) {
	c := writeConfig(t, `
[keybinds]
  up = ["w"]
`)
	if got := c.Keymap.Up.Keys(); !equalKeys(got, []string{"w"}) {
		t.Errorf("Up keys = %v, want [w] (defaults replaced)", got)
	}
}

func TestOmittedActionKeepsDefault(t *testing.T) {
	c := writeConfig(t, `
[keybinds]
  up = ["w"]
`)
	if got := c.Keymap.Down.Keys(); !equalKeys(got, []string{"down", "j"}) {
		t.Errorf("Down keys = %v, want default [down j]", got)
	}
}

func TestEmptyListRevertsToDefault(t *testing.T) {
	c := writeConfig(t, `
[keybinds]
  up = []
`)
	if got := c.Keymap.Up.Keys(); !equalKeys(got, []string{"up", "k"}) {
		t.Errorf("Up keys = %v, want default [up k]", got)
	}
}

func TestInvalidKeyIsDroppedValidKeysSurvive(t *testing.T) {
	c := writeConfig(t, `
[keybinds]
  kill = ["x", "ctrl-k", "Q"]
`)
	// "ctrl-k" uses a hyphen (not the "ctrl+" form) so it is not a recognized
	// key; it is dropped while the surrounding valid keys are kept.
	if got := c.Keymap.Kill.Keys(); !equalKeys(got, []string{"x", "Q"}) {
		t.Errorf("Kill keys = %v, want [x Q] (invalid ctrl-k dropped)", got)
	}
	if !hasWarning(c.Warnings, "ctrl-k") {
		t.Errorf("expected a warning naming the invalid key, warnings = %v", c.Warnings)
	}
}

func TestConflictKeepsKeyOnHigherImportanceAction(t *testing.T) {
	// "k" is up's default key. "down" sits below "up" in the canonical
	// importance order, so when down also claims "k", up keeps it (first-wins)
	// and down keeps its remaining valid key.
	c := writeConfig(t, `
[keybinds]
  down = ["k", "down"]
`)
	if got := c.Keymap.Up.Keys(); !equalKeys(got, []string{"up", "k"}) {
		t.Errorf("Up keys = %v, want [up k] (kept by higher-importance action)", got)
	}
	if got := c.Keymap.Down.Keys(); !equalKeys(got, []string{"down"}) {
		t.Errorf("Down keys = %v, want [down] (k lost to up)", got)
	}
	if !hasWarning(c.Warnings, "up") || !hasWarning(c.Warnings, "down") {
		t.Errorf("expected a warning naming both actions in the conflict, warnings = %v", c.Warnings)
	}
}

func TestReservedKeyCannotBeReboundToAnotherAction(t *testing.T) {
	c := writeConfig(t, `
[keybinds]
  kill = ["enter", "x"]
`)
	// "enter" is a reserved hatch (attach/confirm) and cannot be reassigned;
	// kill keeps its other valid key.
	if got := c.Keymap.Kill.Keys(); !equalKeys(got, []string{"x"}) {
		t.Errorf("Kill keys = %v, want [x] (reserved enter rejected)", got)
	}
	// attach still owns enter, untouched by the hostile rebind.
	if got := c.Keymap.Enter.Keys(); !equalKeys(got, []string{"enter"}) {
		t.Errorf("Enter keys = %v, want [enter] (reserved owner unchanged)", got)
	}
	if !hasWarning(c.Warnings, "enter") || !hasWarning(c.Warnings, "reserved") {
		t.Errorf("expected a reserved-key warning, warnings = %v", c.Warnings)
	}
}

func TestCtrlCIsReservedFromEveryAction(t *testing.T) {
	c := writeConfig(t, `
[keybinds]
  quit = ["ctrl+c", "Q"]
`)
	// ctrl+c is the always-on quit hatch; no action (not even quit) may claim
	// it, so quit keeps only its other valid key.
	if got := c.Keymap.Quit.Keys(); !equalKeys(got, []string{"Q"}) {
		t.Errorf("Quit keys = %v, want [Q] (ctrl+c reserved)", got)
	}
	if !hasWarning(c.Warnings, "ctrl+c") || !hasWarning(c.Warnings, "reserved") {
		t.Errorf("expected ctrl+c reserved warning, warnings = %v", c.Warnings)
	}
}

func TestActionWithAllKeysDroppedRevertsToDefault(t *testing.T) {
	c := writeConfig(t, `
[keybinds]
  kill = ["enter"]
`)
	// The only requested key is reserved and dropped, leaving kill with nothing;
	// it reverts to its default so the action can never be lost entirely.
	if got := c.Keymap.Kill.Keys(); !equalKeys(got, []string{"K", "ctrl+k"}) {
		t.Errorf("Kill keys = %v, want default [K ctrl+k] (all overrides dropped)", got)
	}
}

func TestConflictWarnsExactlyOncePerLostKey(t *testing.T) {
	// help outranks quit and takes quit's only default key "q". quit is not
	// itself overridden, so it must not trigger the override-only fallback and
	// must warn exactly once — no duplicate from a pointless re-claim.
	c := writeConfig(t, `
[keybinds]
  help = ["q"]
`)
	if got := c.Keymap.Help.Keys(); !equalKeys(got, []string{"q"}) {
		t.Errorf("Help keys = %v, want [q] (claimed first)", got)
	}
	n := 0
	for _, w := range c.Warnings {
		if strings.Contains(w, "keybinds.quit") {
			n++
		}
	}
	if n != 1 {
		t.Errorf("quit losing its key should warn exactly once, got %d: %v", n, c.Warnings)
	}
}

func TestDiagnosticsReportConflictWinnerAndLoser(t *testing.T) {
	c := writeConfig(t, `
[keybinds]
  down = ["k", "down"]
`)
	// The conflict over "k" yields a ✗ for down (lost it) and a ✓ for up (kept).
	if !hasDiag(c.Diagnostics, false, "down") {
		t.Errorf("expected a ✗ diagnostic mentioning down, got %+v", c.Diagnostics)
	}
	if !hasDiag(c.Diagnostics, true, "up") {
		t.Errorf("expected a ✓ diagnostic mentioning up (winner), got %+v", c.Diagnostics)
	}
}

func TestDiagnosticsReportThemeFallback(t *testing.T) {
	c := writeConfig(t, `
[ui]
  theme = "missing"
`)
	if !hasDiag(c.Diagnostics, false, "missing") {
		t.Errorf("expected a ✗ diagnostic for the unknown theme, got %+v", c.Diagnostics)
	}
	if !hasDiag(c.Diagnostics, true, "catppuccin-mocha") {
		t.Errorf("expected a ✓ diagnostic for the fallback theme, got %+v", c.Diagnostics)
	}
}

func TestDefaultConfigProducesNoDiagnostics(t *testing.T) {
	dir := t.TempDir()
	c, err := Load(filepath.Join(dir, "config.toml"))
	if err != nil {
		t.Fatal(err)
	}
	if len(c.Diagnostics) != 0 {
		t.Fatalf("default config should produce no diagnostics, got %+v", c.Diagnostics)
	}
}

func hasDiag(diags []Diagnostic, ok bool, substr string) bool {
	for _, d := range diags {
		if d.OK == ok && strings.Contains(d.Detail, substr) {
			return true
		}
	}
	return false
}

func hasWarning(warnings []string, substr string) bool {
	for _, w := range warnings {
		if strings.Contains(w, substr) {
			return true
		}
	}
	return false
}

func equalKeys(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func writeConfig(t *testing.T, body string) Config {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	c, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	return c
}
