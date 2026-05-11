package tmux

import "testing"

func TestFocusHooksMapContainsAllExpectedHooks(t *testing.T) {
	want := map[string]string{
		"client-attached":  "in",
		"client-focus-in":  "in",
		"client-detached":  "out",
		"client-focus-out": "out",
	}
	for hook, wantDir := range want {
		gotDir, ok := focusHooks[hook]
		if !ok {
			t.Errorf("focusHooks missing %q", hook)
			continue
		}
		if gotDir != wantDir {
			t.Errorf("focusHooks[%q] = %q, want %q", hook, gotDir, wantDir)
		}
	}
}
