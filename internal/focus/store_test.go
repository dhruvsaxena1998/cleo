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

func TestReadPrunesStaleEntries(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(filepath.Join(dir, "focus.json"))

	tenMinAgo := time.Now().Add(-10 * time.Minute)
	now := time.Now()

	// Seed file with one stale entry and one fresh entry.
	f := fileFormat{
		Sessions: map[string]sessionFocus{
			"stale-session":        {Focused: true, UpdatedAt: tenMinAgo},
			"fresh-session":        {Focused: true, UpdatedAt: now},
			"stale-false-session":  {Focused: false, UpdatedAt: tenMinAgo},
			"fresh-false-session":  {Focused: false, UpdatedAt: now},
		},
	}
	b, _ := json.MarshalIndent(f, "", "  ")
	if err := os.WriteFile(store.path, b, 0o644); err != nil {
		t.Fatal(err)
	}

	// After a Set() (which calls read + write), stale entries should be gone.
	if err := store.Set("fresh-session", false); err != nil {
		t.Fatal(err)
	}

	// Re-read the file directly to verify stale entries were not persisted.
	raw, err := os.ReadFile(store.path)
	if err != nil {
		t.Fatal(err)
	}
	var onDisk fileFormat
	if err := json.Unmarshal(raw, &onDisk); err != nil {
		t.Fatal(err)
	}

	if _, exists := onDisk.Sessions["stale-session"]; exists {
		t.Error("stale-session should have been pruned from disk after Set()")
	}
	if _, exists := onDisk.Sessions["stale-false-session"]; exists {
		t.Error("stale-false-session should have been pruned from disk after Set()")
	}
	if _, exists := onDisk.Sessions["fresh-session"]; !exists {
		t.Error("fresh-session should still be on disk")
	}
	if _, exists := onDisk.Sessions["fresh-false-session"]; !exists {
		t.Error("fresh-false-session should still be on disk")
	}
}
