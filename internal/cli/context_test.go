package cli

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNewCtxWithRoot(t *testing.T) {
	dir := t.TempDir()
	c, err := NewCtxWithRoot(dir)
	if err != nil {
		t.Fatal(err)
	}
	if c.Paths.ConfigDir() != dir {
		t.Errorf("config dir mismatch")
	}
	if c.Config.DefaultAgent != "claude" {
		t.Errorf("config not loaded")
	}
}

func TestNewCtxWithRootExtractsDefaultSounds(t *testing.T) {
	dir := t.TempDir()
	c, err := NewCtxWithRoot(dir)
	if err != nil {
		t.Fatal(err)
	}

	for _, name := range []string{"start.wav", "attention.wav", "done.wav", "error.wav"} {
		if _, err := os.Stat(filepath.Join(c.Paths.SoundsDir(), name)); err != nil {
			t.Fatalf("default sound %s not extracted: %v", name, err)
		}
	}
}
