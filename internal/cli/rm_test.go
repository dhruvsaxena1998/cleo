package cli

import (
	"path/filepath"
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
