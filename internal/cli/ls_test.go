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
	c.Tmux = &fakeTmux{exists: map[string]bool{"cleo-myapp-claude-1": true}}

	cmd := newLsCmd(func() *Ctx { return c })
	out := &bytes.Buffer{}
	cmd.SetOut(out)
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "myapp") || !strings.Contains(out.String(), "running") {
		t.Errorf("output: %q", out.String())
	}
}

func TestLsReconcilesMissingSessions(t *testing.T) {
	root := t.TempDir()
	c, _ := NewCtxWithRoot(root)
	target := filepath.Join(t.TempDir(), "myapp")
	_ = mkdir(target)
	_, _ = c.Projects.Add(target)

	// Seed a running session that's NOT in fake tmux's exists set.
	_ = c.State.Put(state.Session{
		ID: "cleo-myapp-claude-ghost", ProjectID: "myapp", Agent: "claude",
		Name: "ghost", State: state.Running,
	})
	c.Tmux = &fakeTmux{exists: map[string]bool{}} // empty: ghost is missing from tmux

	cmd := newLsCmd(func() *Ctx { return c })
	out := &bytes.Buffer{}
	cmd.SetOut(out)
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	got, _ := c.State.Get("cleo-myapp-claude-ghost")
	if got.State != state.Dead {
		t.Errorf("expected dead after reconcile, got %s", got.State)
	}
}
