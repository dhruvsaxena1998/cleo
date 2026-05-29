package hooks

import "testing"

// setTestHome roots every protocol's config tree (~/.claude, ~/.codex, ~/.pi,
// ~/.config/opencode) under dir for the duration of the test, then restores
// the previous value. Replaces the old per-protocol directory overrides.
func setTestHome(t *testing.T, dir string) {
	t.Helper()
	orig := testHomeDir
	testHomeDir = dir
	t.Cleanup(func() { testHomeDir = orig })
}
