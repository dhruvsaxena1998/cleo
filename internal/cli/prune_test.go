package cli

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/dhruvsaxena1998/cleo/internal/state"
)

func TestPruneArchivesFinishedSessions(t *testing.T) {
	root := t.TempDir()
	c, _ := NewCtxWithRoot(root)
	for _, st := range []state.State{state.Completed, state.Errored, state.Dead, state.Running, state.Idle} {
		_ = c.State.Put(state.Session{
			ID:        "cleo-foo-claude-" + string(st),
			ProjectID: "foo", Agent: "claude", Name: string(st), State: st,
		})
	}
	cmd := newPruneCmd(func() *Ctx { return c })
	cmd.SetArgs([]string{"foo", "--keep", "0", "--yes"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	all, _ := c.State.List()
	for _, s := range all {
		if s.State.IsFinished() {
			t.Errorf("finished still present: %s", s.ID)
		}
	}
}

func TestPruneSkipsDirtyWorktreeAndForceOverrides(t *testing.T) {
	root := t.TempDir()
	c, _ := NewCtxWithRoot(root)
	wt := &fakeWorktree{dirty: map[string]bool{"/x/foo/.cleo/worktrees/claude-wip": true}}
	c.Worktree = wt
	_ = c.State.Put(state.Session{
		ID: "cleo-foo-claude-wip", ProjectID: "foo", Agent: "claude", Name: "wip", State: state.Dead,
		WorktreePath: "/x/foo/.cleo/worktrees/claude-wip", WorktreeBranch: "cleo/wt-claude-wip",
	})

	cmd := newPruneCmd(func() *Ctx { return c })
	cmd.SetArgs([]string{"foo", "--keep", "0", "--yes"})
	out := &bytes.Buffer{}
	cmd.SetOut(out)
	cmd.SetErr(out)
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if _, err := c.State.Get("cleo-foo-claude-wip"); err != nil {
		t.Fatalf("dirty worktree session must be kept: %v", err)
	}
	if !strings.Contains(out.String(), "claude-wip") {
		t.Fatalf("output %q should warn about the skipped worktree", out.String())
	}

	cmd = newPruneCmd(func() *Ctx { return c })
	cmd.SetArgs([]string{"foo", "--keep", "0", "--yes", "--force"})
	cmd.SetOut(&bytes.Buffer{})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if _, err := c.State.Get("cleo-foo-claude-wip"); err == nil {
		t.Fatal("forced prune should remove the dirty worktree session")
	}
	if len(wt.removed) != 1 || !wt.removed[0].Force {
		t.Fatalf("removals = %#v, want one forced", wt.removed)
	}
}

func TestPruneDryRunUsesConfiguredKeepDefault(t *testing.T) {
	root := t.TempDir()
	c, _ := NewCtxWithRoot(root)
	c.Config.Pruning.KeepDefault = 1
	now := time.Now()
	for i, name := range []string{"oldest", "middle", "newest"} {
		_ = c.State.Put(state.Session{
			ID:          "cleo-foo-claude-" + name,
			ProjectID:   "foo",
			Agent:       "claude",
			Name:        name,
			State:       state.Completed,
			LastEventAt: now.Add(time.Duration(i) * time.Minute),
		})
	}
	cmd := newPruneCmd(func() *Ctx { return c })
	cmd.SetArgs([]string{"foo", "--dry-run", "--yes"})
	var out bytes.Buffer
	cmd.SetOut(&out)

	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}

	got := out.String()
	if strings.Contains(got, "newest") {
		t.Fatalf("dry run should keep newest session, got %q", got)
	}
	for _, want := range []string{"oldest", "middle"} {
		if !strings.Contains(got, want) {
			t.Fatalf("dry run missing %q in %q", want, got)
		}
	}
}
