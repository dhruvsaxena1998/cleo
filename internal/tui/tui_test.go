package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/exp/teatest"

	"github.com/dhruvsaxena1998/cleo/internal/cli"
	"github.com/dhruvsaxena1998/cleo/internal/state"
	"github.com/dhruvsaxena1998/cleo/internal/tmux"
)

func mkdirAll(p string) error { return os.MkdirAll(p, 0o755) }

func contains(b []byte, s string) bool {
	return strings.Contains(string(b), s)
}

// fakeTmux is a test double for cli.TmuxClient that reports a fixed set of live sessions.
type fakeTmux struct {
	live map[string]bool
}

func (f *fakeTmux) NewSession(_ tmux.NewSessionOpts) error { return nil }
func (f *fakeTmux) HasSession(n string) (bool, error)      { return f.live[n], nil }
func (f *fakeTmux) LsPrefix(prefix string) ([]string, error) {
	var out []string
	for n := range f.live {
		if strings.HasPrefix(n, prefix) {
			out = append(out, n)
		}
	}
	return out, nil
}
func (f *fakeTmux) Kill(n string) error                     { delete(f.live, n); return nil }
func (f *fakeTmux) CapturePane(string, int) (string, error) { return "", nil }
func (f *fakeTmux) RenameSession(from, to string) error     { return nil }

func TestRenamePopupOpensAndUpdatesSessionName(t *testing.T) {
	root := t.TempDir()
	c, err := cli.NewCtxWithRoot(root)
	if err != nil {
		t.Fatal(err)
	}
	c.Tmux = &fakeTmux{live: map[string]bool{"cleo-myapp-claude-1": true}}
	target := filepath.Join(t.TempDir(), "myapp")
	_ = os.MkdirAll(target, 0o755)
	if _, err := c.Projects.Add(target); err != nil {
		t.Fatal(err)
	}
	_ = c.State.Put(state.Session{
		ID: "cleo-myapp-claude-1", ProjectID: "myapp", Agent: "claude",
		Name: "1", State: state.Running, StartedAt: time.Now(),
	})

	m := New(c)
	tm := teatest.NewTestModel(t, m, teatest.WithInitialTermSize(120, 40))

	// Wait for session to render, then navigate to it and press r
	teatest.WaitFor(t, tm.Output(), func(b []byte) bool {
		return contains(b, "cl") && contains(b, "run")
	}, teatest.WithDuration(3*time.Second))

	// Navigate down to the session row (project is expanded with one session)
	tm.Send(tea.KeyMsg{Type: tea.KeyDown})
	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("r")})

	// Rename popup should appear
	teatest.WaitFor(t, tm.Output(), func(b []byte) bool {
		return contains(b, "Rename Session")
	}, teatest.WithDuration(2*time.Second))

	// Clear pre-filled name, type new name, confirm
	tm.Send(tea.KeyMsg{Type: tea.KeyCtrlU}) // kill line — clears the text input
	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("my-task")})
	tm.Send(tea.KeyMsg{Type: tea.KeyEnter})

	// Verify the session name was updated in the store
	teatest.WaitFor(t, tm.Output(), func(b []byte) bool {
		sess, err := c.State.Get("cleo-myapp-claude-1")
		return err == nil && sess.Name == "my-task"
	}, teatest.WithDuration(2*time.Second))

	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})
	tm.WaitFinished(t)
}

func TestSidebarRendersProjectsAndSessions(t *testing.T) {
	root := t.TempDir()
	c, err := cli.NewCtxWithRoot(root)
	if err != nil {
		t.Fatal(err)
	}
	// Use a fake tmux so reconcile.Run sees the session as live and doesn't mark it Dead.
	c.Tmux = &fakeTmux{live: map[string]bool{"cleo-myapp-claude-1": true}}
	target := filepath.Join(t.TempDir(), "myapp")
	if err := mkdirAll(target); err != nil {
		t.Fatal(err)
	}
	if _, err := c.Projects.Add(target); err != nil {
		t.Fatal(err)
	}
	_ = c.State.Put(state.Session{
		ID: "cleo-myapp-claude-1", ProjectID: "myapp", Agent: "claude",
		Name: "1", State: state.Running, StartedAt: time.Now(),
	})

	m := New(c)
	tm := teatest.NewTestModel(t, m, teatest.WithInitialTermSize(120, 40))

	// Auto-expand fires on first state load; just wait for agent and state to appear.
	teatest.WaitFor(t, tm.Output(), func(b []byte) bool {
		return contains(b, "cl") && contains(b, "run")
	}, teatest.WithDuration(3*time.Second))

	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})
	tm.WaitFinished(t)
}

func TestEnterOnDeadSessionDoesNotAttach(t *testing.T) {
	root := t.TempDir()
	c, err := cli.NewCtxWithRoot(root)
	if err != nil {
		t.Fatal(err)
	}
	c.Tmux = &fakeTmux{live: map[string]bool{}}
	target := filepath.Join(t.TempDir(), "myapp")
	if err := mkdirAll(target); err != nil {
		t.Fatal(err)
	}
	if _, err := c.Projects.Add(target); err != nil {
		t.Fatal(err)
	}
	_ = c.State.Put(state.Session{
		ID: "cleo-myapp-codex-1", ProjectID: "myapp", Agent: "codex",
		Name: "1", State: state.Dead, StartedAt: time.Now(), LastEventAt: time.Now(),
	})

	m := New(c)
	m.projects, _ = c.Projects.List()
	m.sessions, _ = c.State.List()
	m.expanded["myapp"] = true
	m.cursor.projectIdx = 0
	m.cursor.agentIdx = 0

	got, cmd := m.attachSelectedAgent()
	if cmd != nil {
		t.Fatal("dead session should not produce an attach command")
	}
	if !strings.Contains(got.status, "press K") {
		t.Fatalf("expected remove hint status, got %q", got.status)
	}
}
