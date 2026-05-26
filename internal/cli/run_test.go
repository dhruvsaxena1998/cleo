package cli

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dhruvsaxena1998/cleo/internal/state"
)

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
