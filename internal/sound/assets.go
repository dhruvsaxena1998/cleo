package sound

import (
	"embed"
	"io"
	"os"
	"path/filepath"
)

//go:embed all:assets
var assetsFS embed.FS

// ExtractDefaults copies bundled WAVs to dir if not already present.
func ExtractDefaults(dir string) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	entries, err := assetsFS.ReadDir("assets")
	if err != nil {
		return err
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		dst := filepath.Join(dir, e.Name())
		if _, err := os.Stat(dst); err == nil {
			continue // idempotent
		}
		src, err := assetsFS.Open("assets/" + e.Name())
		if err != nil {
			return err
		}
		out, err := os.Create(dst)
		if err != nil {
			src.Close()
			return err
		}
		if _, err := io.Copy(out, src); err != nil {
			src.Close()
			out.Close()
			return err
		}
		src.Close()
		out.Close()
	}
	return nil
}
