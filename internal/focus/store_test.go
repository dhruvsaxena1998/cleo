package focus

import (
	"path/filepath"
	"testing"
)

func TestStoreTracksFocusedSessions(t *testing.T) {
	store := NewStore(filepath.Join(t.TempDir(), "focus.json"))

	if store.IsFocused("cleo-myapp-codex-1") {
		t.Fatal("missing session should not be focused")
	}
	if err := store.Set("cleo-myapp-codex-1", true); err != nil {
		t.Fatal(err)
	}
	if !store.IsFocused("cleo-myapp-codex-1") {
		t.Fatal("session should be focused")
	}
	if err := store.Set("cleo-myapp-codex-1", false); err != nil {
		t.Fatal(err)
	}
	if store.IsFocused("cleo-myapp-codex-1") {
		t.Fatal("session should not be focused")
	}
}
