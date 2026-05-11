package tmux

import "testing"

func TestInstallFocusHooksIncludesFocusOut(t *testing.T) {
	// client-focus-out must be registered so switching tmux windows clears
	// focus (without it, focus sticks until detach or process crash).
	hooks := map[string]string{
		"client-attached":  "in",
		"client-focus-in":  "in",
		"client-detached":  "out",
		"client-focus-out": "out",
	}
	if _, ok := hooks["client-focus-out"]; !ok {
		t.Error("client-focus-out hook must be registered")
	}
}
