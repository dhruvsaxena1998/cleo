package sound

import (
	"os"
	"path/filepath"
	"testing"
)

func TestExtractAssetsCreatesFiles(t *testing.T) {
	dir := t.TempDir()
	if err := ExtractDefaults(dir); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"start.wav", "attention.wav", "done.wav", "error.wav"} {
		if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
			t.Errorf("missing %s: %v", name, err)
		}
	}
}

func TestExtractAssetsIdempotent(t *testing.T) {
	dir := t.TempDir()
	if err := ExtractDefaults(dir); err != nil {
		t.Fatal(err)
	}
	// second call must not error or overwrite if file exists
	if err := ExtractDefaults(dir); err != nil {
		t.Fatal(err)
	}
}

func TestProbePlayerNoneFound(t *testing.T) {
	// override $PATH so no player is found
	t.Setenv("PATH", "")
	p := NewPlayer(0.7)
	if p.Available() {
		t.Errorf("expected unavailable")
	}
	// Play() must not return error even when nothing is available
	if err := p.Play("/nope.wav"); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}
