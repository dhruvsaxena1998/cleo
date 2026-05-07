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

	// Wait for initial state to load
	teatest.WaitFor(t, tm.Output(), func(b []byte) bool {
		return contains(b, "myapp")
	}, teatest.WithDuration(3*time.Second))

	// expand the project (space key)
	tm.Send(tea.KeyMsg{Type: tea.KeySpace})

	teatest.WaitFor(t, tm.Output(), func(b []byte) bool {
		return contains(b, "[cl]") && contains(b, "running")
	}, teatest.WithDuration(3*time.Second))

	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})
	tm.WaitFinished(t)
}
