package cli

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dhruvsaxena1998/cleo/internal/state"
)

func TestRmRemovesProject(t *testing.T) {
	root := t.TempDir()
	c, _ := NewCtxWithRoot(root)
	target := filepath.Join(t.TempDir(), "myapp")
	_ = mkdir(target)
	_, _ = c.Projects.Add(target)

	cmd := newRmCmd(testRootedCtx(t, root))
	cmd.SetArgs([]string{"myapp", "--yes"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if _, err := c.Projects.Get("myapp"); err == nil {
		t.Errorf("expected gone")
	}
}

func TestRmByPath(t *testing.T) {
	root := t.TempDir()
	c, _ := NewCtxWithRoot(root)
	target := filepath.Join(t.TempDir(), "myapp")
	_ = mkdir(target)
	_, _ = c.Projects.Add(target)

	cmd := newRmCmd(testRootedCtx(t, root))
	cmd.SetArgs([]string{target, "--yes"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if _, err := c.Projects.Get("myapp"); err == nil {
		t.Errorf("expected gone after path-based remove")
	}
}

func TestRmSessionIDRemovesSessionRecordAndWorktree(t *testing.T) {
	root := t.TempDir()
	c, _ := NewCtxWithRoot(root)
	target := filepath.Join(t.TempDir(), "myapp")
	_ = mkdir(target)
	_, _ = c.Projects.Add(target)
	wtDir := filepath.Join(target, ".cleo", "worktrees", "claude-done")
	_ = c.State.Put(state.Session{
		ID: "cleo-myapp-claude-done", ProjectID: "myapp", Agent: "claude", Name: "done",
		State: state.Dead, WorktreePath: wtDir, WorktreeBranch: "cleo/wt-claude-done",
	})
	wt := &fakeWorktree{}
	c.Worktree = wt

	cmd := newRmCmd(func() *Ctx { return c })
	cmd.SetArgs([]string{"cleo-myapp-claude-done", "--yes"})
	out := &bytes.Buffer{}
	cmd.SetOut(out)
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}

	if _, err := c.State.Get("cleo-myapp-claude-done"); err == nil {
		t.Fatal("session record should be gone")
	}
	if len(wt.removed) != 1 || wt.removed[0].Dir != wtDir {
		t.Fatalf("worktree removals = %#v, want %q", wt.removed, wtDir)
	}
	// The project itself must be untouched: rm <session-id> is session removal.
	if _, err := c.Projects.Get("myapp"); err != nil {
		t.Fatalf("project should survive session rm: %v", err)
	}
}

func TestRmSessionIDWithDirtyWorktreeFailsWithForceHint(t *testing.T) {
	root := t.TempDir()
	c, _ := NewCtxWithRoot(root)
	target := filepath.Join(t.TempDir(), "myapp")
	_ = mkdir(target)
	_, _ = c.Projects.Add(target)
	wtDir := filepath.Join(target, ".cleo", "worktrees", "claude-wip")
	_ = c.State.Put(state.Session{
		ID: "cleo-myapp-claude-wip", ProjectID: "myapp", Agent: "claude", Name: "wip",
		State: state.Dead, WorktreePath: wtDir, WorktreeBranch: "cleo/wt-claude-wip",
	})
	wt := &fakeWorktree{dirty: map[string]bool{wtDir: true}}
	c.Worktree = wt

	cmd := newRmCmd(func() *Ctx { return c })
	cmd.SetArgs([]string{"cleo-myapp-claude-wip", "--yes"})
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "--force") {
		t.Fatalf("error = %v, want dirty-worktree failure hinting --force", err)
	}
	if _, getErr := c.State.Get("cleo-myapp-claude-wip"); getErr != nil {
		t.Fatalf("dirty session record must be kept: %v", getErr)
	}
	if len(wt.removed) != 0 {
		t.Fatalf("dirty worktree must not be removed: %#v", wt.removed)
	}
}

func TestRmProjectAbortsListingDirtyWorktreeOffenders(t *testing.T) {
	root := t.TempDir()
	c, _ := NewCtxWithRoot(root)
	target := filepath.Join(t.TempDir(), "myapp")
	_ = mkdir(target)
	_, _ = c.Projects.Add(target)
	wtDir := filepath.Join(target, ".cleo", "worktrees", "claude-wip")
	_ = c.State.Put(state.Session{
		ID: "cleo-myapp-claude-wip", ProjectID: "myapp", Agent: "claude", Name: "wip",
		State: state.Dead, WorktreePath: wtDir, WorktreeBranch: "cleo/wt-claude-wip",
	})
	wt := &fakeWorktree{dirty: map[string]bool{wtDir: true}}
	c.Worktree = wt

	cmd := newRmCmd(func() *Ctx { return c })
	cmd.SetArgs([]string{"myapp", "--yes"})
	out := &bytes.Buffer{}
	cmd.SetOut(out)
	cmd.SetErr(out)
	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error()+out.String(), "cleo-myapp-claude-wip") {
		t.Fatalf("err=%v out=%q, want the offending session listed", err, out.String())
	}
	if _, err := c.Projects.Get("myapp"); err != nil {
		t.Fatalf("aborted removal must keep the project: %v", err)
	}
	if _, err := c.State.Get("cleo-myapp-claude-wip"); err != nil {
		t.Fatalf("aborted removal must keep the session record: %v", err)
	}
}

func TestRmBlocksActiveSession(t *testing.T) {
	root := t.TempDir()
	c, _ := NewCtxWithRoot(root)
	target := filepath.Join(t.TempDir(), "myapp")
	_ = mkdir(target)
	_, _ = c.Projects.Add(target)
	_ = c.State.Put(state.Session{ID: "cleo-myapp-claude-1", ProjectID: "myapp", State: state.Running})

	cmd := newRmCmd(testRootedCtx(t, root))
	cmd.SetArgs([]string{"myapp", "--yes"})
	if err := cmd.Execute(); err == nil {
		t.Fatal("expected error for active session")
	}
}

func TestRmForceRemovesWithActiveSessions(t *testing.T) {
	root := t.TempDir()
	c, _ := NewCtxWithRoot(root)
	target := filepath.Join(t.TempDir(), "myapp")
	_ = mkdir(target)
	_, _ = c.Projects.Add(target)
	_ = c.State.Put(state.Session{ID: "cleo-myapp-claude-1", ProjectID: "myapp", State: state.Running})
	_ = c.State.Put(state.Session{ID: "cleo-myapp-claude-2", ProjectID: "myapp", State: state.Completed})

	cmd := newRmCmd(testRootedCtx(t, root))
	cmd.SetArgs([]string{"myapp", "--yes", "--force"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if _, err := c.Projects.Get("myapp"); err == nil {
		t.Errorf("expected project gone")
	}
	sessions, _ := c.State.List()
	for _, s := range sessions {
		if s.ProjectID == "myapp" {
			t.Errorf("expected session %s gone", s.ID)
		}
	}
}

func TestRmCleansUpFinishedSessions(t *testing.T) {
	root := t.TempDir()
	c, _ := NewCtxWithRoot(root)
	target := filepath.Join(t.TempDir(), "myapp")
	_ = mkdir(target)
	_, _ = c.Projects.Add(target)
	_ = c.State.Put(state.Session{ID: "cleo-myapp-claude-1", ProjectID: "myapp", State: state.Completed})
	_ = c.State.Put(state.Session{ID: "cleo-myapp-claude-2", ProjectID: "myapp", State: state.Dead})

	cmd := newRmCmd(testRootedCtx(t, root))
	cmd.SetArgs([]string{"myapp", "--yes"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	sessions, _ := c.State.List()
	for _, s := range sessions {
		if s.ProjectID == "myapp" {
			t.Errorf("expected session %s cleaned up", s.ID)
		}
	}
}
