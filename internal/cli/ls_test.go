package cli

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dhruvsaxena1998/cleo/internal/state"
)

func TestLsShowsProjectsAndSessions(t *testing.T) {
	root := t.TempDir()
	c, _ := NewCtxWithRoot(root)
	target := filepath.Join(t.TempDir(), "myapp")
	_ = mkdir(target)
	_, _ = c.Projects.Add(target)
	_ = c.State.Put(state.Session{ID: "cleo-myapp-claude-1", ProjectID: "myapp", Agent: "claude", Name: "1", State: state.Running})

	cmd := newLsCmd(testRootedCtx(t, root))
	out := &bytes.Buffer{}
	cmd.SetOut(out)
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "myapp") || !strings.Contains(out.String(), "running") {
		t.Errorf("output: %q", out.String())
	}
}
