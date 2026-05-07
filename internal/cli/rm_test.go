package cli

import (
	"path/filepath"
	"testing"
)

func TestRmRemovesProject(t *testing.T) {
	root := t.TempDir()
	c, _ := NewCtxWithRoot(root)
	target := filepath.Join(t.TempDir(), "myapp")
	_ = mkdir(target)
	_, _ = c.Projects.Add(target)

	cmd := newRmCmd(testRootedCtx(t, root))
	cmd.SetArgs([]string{"myapp"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if _, err := c.Projects.Get("myapp"); err == nil {
		t.Errorf("expected gone")
	}
}
