package cli

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dhruvsaxena1998/cleo/internal/state"
)

// TestRunRoutesAttachThroughSeam drops --no-attach and proves spawn-and-attach
// obtains its attach command from the Tmux seam (so the socket is honored and
// attach is finally mockable) instead of hand-building a raw tmux command that
// goes around the adapter.
func TestRunRoutesAttachThroughSeam(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(t.TempDir(), "myapp")
	_ = mkdir(target)

	c, _ := NewCtxWithRoot(root)
	usePortableAgentCommand(c, "claude")
	_, _ = c.Projects.Add(target)

	fake := &fakeTmux{}
	c.Tmux = fake

	cmd := newRunCmd(func() *Ctx { return c })
	cmd.SetArgs([]string{"claude", "--name", "fix-auth-bug", "--cwd", target, "--yes"})
	cmd.SetOut(&bytes.Buffer{})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}

	wantID := "cleo-myapp-claude-fix-auth-bug"
	if len(fake.attached) != 1 || fake.attached[0] != wantID {
		t.Fatalf("expected attach requested for %q via the seam, got %v", wantID, fake.attached)
	}
}

func TestRunSpawnsAndRecordsSession(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(t.TempDir(), "myapp")
	_ = mkdir(target)

	c, _ := NewCtxWithRoot(root)
	usePortableAgentCommand(c, "claude")
	_, _ = c.Projects.Add(target)

	// Use a fake tmux that records calls instead of running the binary.
	fake := &fakeTmux{}
	c.Tmux = fake
	getCtx := func() *Ctx { return c }

	cmd := newRunCmd(getCtx)
	cmd.SetArgs([]string{"claude", "--name", "fix-auth-bug", "--cwd", target, "--yes"})
	out := &bytes.Buffer{}
	cmd.SetOut(out)
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if len(fake.created) != 1 {
		t.Fatalf("expected one session created, got %d", len(fake.created))
	}
	if fake.created[0].Name != "cleo-myapp-claude-fix-auth-bug" {
		t.Errorf("session name: %q", fake.created[0].Name)
	}
	if fake.created[0].Env["CLEO_SESSION_ID"] != "cleo-myapp-claude-fix-auth-bug" {
		t.Errorf("env not set")
	}
	got, err := c.State.Get("cleo-myapp-claude-fix-auth-bug")
	if err != nil {
		t.Fatal(err)
	}
	if got.State != state.Spawning {
		t.Errorf("state: %s", got.State)
	}
}

func TestRunWithoutNameUsesDockerStyleGeneratedName(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(t.TempDir(), "myapp")
	_ = mkdir(target)

	c, _ := NewCtxWithRoot(root)
	usePortableAgentCommand(c, "claude")
	_, _ = c.Projects.Add(target)
	fake := &fakeTmux{}
	c.Tmux = fake

	cmd := newRunCmd(func() *Ctx { return c })
	cmd.SetArgs([]string{"claude", "--cwd", target, "--yes", "--no-attach"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if len(fake.created) != 1 {
		t.Fatalf("expected one session created, got %d", len(fake.created))
	}
	got := strings.TrimPrefix(fake.created[0].Name, "cleo-myapp-claude-")
	if got == fake.created[0].Name || !strings.Contains(got, "-") {
		t.Fatalf("expected generated docker-style label in %q", fake.created[0].Name)
	}
	if got == "1" || got == "2" {
		t.Fatalf("expected non-numeric generated label, got %q", got)
	}
}

func TestRunWorktreeFlagSpawnsIntoWorktree(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(t.TempDir(), "myapp")
	_ = mkdir(target)

	c, _ := NewCtxWithRoot(root)
	usePortableAgentCommand(c, "claude")
	_, _ = c.Projects.Add(target)
	fake := &fakeTmux{}
	wt := &fakeWorktree{}
	c.Tmux = fake
	c.Worktree = wt

	cmd := newRunCmd(func() *Ctx { return c })
	cmd.SetArgs([]string{"claude", "--name", "isolated", "--cwd", target, "--yes", "--no-attach", "--worktree"})
	out := &bytes.Buffer{}
	cmd.SetOut(out)
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}

	wantDir := filepath.Join(target, ".cleo", "worktrees", "claude-isolated")
	if len(wt.created) != 1 || wt.created[0].Dir != wantDir {
		t.Fatalf("worktree created = %#v, want dir %q", wt.created, wantDir)
	}
	if len(fake.created) != 1 || fake.created[0].Cwd != wantDir {
		t.Fatalf("tmux cwd = %#v, want the worktree", fake.created)
	}
	got, err := c.State.Get("cleo-myapp-claude-isolated")
	if err != nil {
		t.Fatal(err)
	}
	if got.WorktreeBranch != "cleo/wt-claude-isolated" {
		t.Fatalf("stored branch = %q", got.WorktreeBranch)
	}
	if !strings.Contains(out.String(), "cleo/wt-claude-isolated") {
		t.Fatalf("run output should name the worktree branch, got %q", out.String())
	}
}

func TestRunBaseFlagIsPassedToWorktreeCreation(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(t.TempDir(), "myapp")
	_ = mkdir(target)

	c, _ := NewCtxWithRoot(root)
	usePortableAgentCommand(c, "claude")
	_, _ = c.Projects.Add(target)
	wt := &fakeWorktree{}
	c.Tmux = &fakeTmux{}
	c.Worktree = wt

	cmd := newRunCmd(func() *Ctx { return c })
	cmd.SetArgs([]string{"claude", "--cwd", target, "--yes", "--no-attach", "--worktree", "--base", "main"})
	cmd.SetOut(&bytes.Buffer{})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if len(wt.created) != 1 || wt.created[0].Base != "main" {
		t.Fatalf("worktree created = %#v, want base main", wt.created)
	}
}

func TestRunRejectsWorktreeAndNoWorktreeTogether(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(t.TempDir(), "myapp")
	_ = mkdir(target)

	c, _ := NewCtxWithRoot(root)
	usePortableAgentCommand(c, "claude")
	_, _ = c.Projects.Add(target)
	c.Tmux = &fakeTmux{}
	c.Worktree = &fakeWorktree{}

	cmd := newRunCmd(func() *Ctx { return c })
	cmd.SetArgs([]string{"claude", "--cwd", target, "--yes", "--no-attach", "--worktree", "--no-worktree"})
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	if err := cmd.Execute(); err == nil {
		t.Fatal("expected an error for --worktree with --no-worktree")
	}
}

func TestRunNoWorktreeOverridesProjectDefault(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(t.TempDir(), "myapp")
	_ = mkdir(target)

	c, _ := NewCtxWithRoot(root)
	usePortableAgentCommand(c, "claude")
	proj, _ := c.Projects.Add(target)
	setProjectDefaultWorktree(t, c, proj.ID)
	wt := &fakeWorktree{}
	c.Tmux = &fakeTmux{}
	c.Worktree = wt

	cmd := newRunCmd(func() *Ctx { return c })
	cmd.SetArgs([]string{"claude", "--cwd", target, "--yes", "--no-attach", "--no-worktree"})
	cmd.SetOut(&bytes.Buffer{})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if len(wt.created) != 0 {
		t.Fatalf("--no-worktree should suppress the project default, got %#v", wt.created)
	}
}

func TestRunUsesConfiguredAgentCommand(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(t.TempDir(), "myapp")
	_ = mkdir(target)

	c, _ := NewCtxWithRoot(root)
	_, _ = c.Projects.Add(target)
	agent := c.Config.Agents["claude"]
	agent.Command = "echo hello-from-cleo"
	c.Config.Agents["claude"] = agent
	fake := &fakeTmux{}
	c.Tmux = fake

	cmd := newRunCmd(func() *Ctx { return c })
	cmd.SetArgs([]string{"claude", "--cwd", target, "--yes", "--no-attach"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if len(fake.created) != 1 {
		t.Fatalf("expected one session created, got %d", len(fake.created))
	}
	if fake.created[0].Cmd != "echo hello-from-cleo" {
		t.Fatalf("tmux command = %q", fake.created[0].Cmd)
	}
}
