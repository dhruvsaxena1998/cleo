package events

import (
	"compress/gzip"
	"io"
	"os"
	"path/filepath"
)

// Archive gzips srcPath and moves it into archiveDir; deletes the src on success.
func Archive(srcPath, archiveDir string) error {
	if _, err := os.Stat(srcPath); os.IsNotExist(err) {
		return nil
	}
	if err := os.MkdirAll(archiveDir, 0o755); err != nil {
		return err
	}
	dst := filepath.Join(archiveDir, filepath.Base(srcPath)+".gz")
	in, err := os.Open(srcPath)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()
	gz := gzip.NewWriter(out)
	if _, err := io.Copy(gz, in); err != nil {
		_ = gz.Close()
		return err
	}
	if err := gz.Close(); err != nil {
		return err
	}
	return os.Remove(srcPath)
}
