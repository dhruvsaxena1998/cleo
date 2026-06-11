package cli

import (
	"bytes"
	"strings"
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

func TestKillWorktreeSessionKeepsRecordAndSaysWherTheWorkIs(t *testing.T) {
	root := t.TempDir()
	c, _ := NewCtxWithRoot(root)
	c.Tmux = &fakeTmux{exists: map[string]bool{"cleo-foo-claude-wt": true}}
	c.Worktree = &fakeWorktree{}
	_ = c.State.Put(state.Session{
		ID: "cleo-foo-claude-wt", State: state.Running,
		WorktreePath: "/x/foo/.cleo/worktrees/claude-wt", WorktreeBranch: "cleo/wt-claude-wt",
	})

	cmd := newKillCmd(func() *Ctx { return c })
	cmd.SetArgs([]string{"cleo-foo-claude-wt", "--yes"})
	out := &bytes.Buffer{}
	cmd.SetOut(out)
	cmd.SetErr(out)
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if got, err := c.State.Get("cleo-foo-claude-wt"); err != nil || got.State != state.Dead {
		t.Fatalf("record = %#v err=%v, want kept and dead", got, err)
	}
	if !strings.Contains(out.String(), "/x/foo/.cleo/worktrees/claude-wt") {
		t.Fatalf("output %q should say where the preserved worktree is", out.String())
	}
}
