package cli

import (
	"testing"

	"github.com/dhruvsaxena1998/cleo/internal/state"
)

func TestKillRemovesSessionFromState(t *testing.T) {
	root := t.TempDir()
	c, _ := NewCtxWithRoot(root)
	c.Tmux = &fakeTmux{exists: map[string]bool{"cleo-foo-claude-1": true}}
	_ = c.State.Put(state.Session{ID: "cleo-foo-claude-1", State: state.Running})

	cmd := newKillCmd(func() *Ctx { return c })
	cmd.SetArgs([]string{"cleo-foo-claude-1", "--yes"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if _, err := c.State.Get("cleo-foo-claude-1"); err != state.ErrSessionNotFound {
		t.Errorf("expected gone")
	}
}
