package config

import (
	"os"
	"path/filepath"
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
