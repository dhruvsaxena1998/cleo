package focus

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
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

func TestIsFocusedReturnsFalseWhenStale(t *testing.T) {
	store := NewStore(filepath.Join(t.TempDir(), "focus.json"))

	staleTime := time.Now().Add(-2 * time.Hour)
	f := fileFormat{
		Sessions: map[string]sessionFocus{
			"cleo-app-claude-1": {Focused: true, UpdatedAt: staleTime},
		},
	}
	b, _ := json.MarshalIndent(f, "", "  ")
	_ = os.WriteFile(store.path, b, 0o644)

	if store.IsFocused("cleo-app-claude-1") {
		t.Error("focused=true with UpdatedAt 2h ago should be treated as stale")
	}
}

func TestIsFocusedReturnsTrueWhenFresh(t *testing.T) {
	store := NewStore(filepath.Join(t.TempDir(), "focus.json"))
	if err := store.Set("cleo-app-claude-1", true); err != nil {
		t.Fatal(err)
	}
	if !store.IsFocused("cleo-app-claude-1") {
		t.Error("just-set focused session should return true")
	}
}

func TestFocusTTLIsUnder10Minutes(t *testing.T) {
	store := NewStore(filepath.Join(t.TempDir(), "focus.json"))

	// Simulate a session that was focused 6 minutes ago
	staleTime := time.Now().Add(-6 * time.Minute)
	f := fileFormat{
		Sessions: map[string]sessionFocus{
			"cleo-app-claude-1": {Focused: true, UpdatedAt: staleTime},
		},
	}
	b, _ := json.MarshalIndent(f, "", "  ")
	_ = os.WriteFile(store.path, b, 0o644)

	if store.IsFocused("cleo-app-claude-1") {
		t.Error("focused=true with UpdatedAt 6 min ago should be treated as stale (TTL must be <= 5 min)")
	}
}
