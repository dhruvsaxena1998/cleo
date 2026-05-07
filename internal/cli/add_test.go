package cli

import (
	"bytes"
	"path/filepath"
	"testing"
)

func TestAddRegistersProject(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(t.TempDir(), "myapp")
	_ = mkdir(target)

	cmd := newAddCmd(testRootedCtx(t, root))
	cmd.SetArgs([]string{target})
	out := &bytes.Buffer{}
	cmd.SetOut(out)
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	c, _ := NewCtxWithRoot(root)
	got, err := c.Projects.Get("myapp")
	if err != nil {
		t.Fatal(err)
	}
	if got.Path != target {
		t.Errorf("path %q", got.Path)
	}
}
