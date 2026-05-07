package cli

import "testing"

func TestNewCtxWithRoot(t *testing.T) {
	dir := t.TempDir()
	c, err := NewCtxWithRoot(dir)
	if err != nil {
		t.Fatal(err)
	}
	if c.Paths.ConfigDir() != dir {
		t.Errorf("config dir mismatch")
	}
	if c.Config.Defaults.DefaultAgent != "claude" {
		t.Errorf("config not loaded")
	}
}
