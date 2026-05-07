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
)

func mkdirAll(p string) error { return os.MkdirAll(p, 0o755) }

func contains(b []byte, s string) bool {
	return strings.Contains(string(b), s)
}

func TestSidebarRendersProjectsAndSessions(t *testing.T) {
	root := t.TempDir()
	c, err := cli.NewCtxWithRoot(root)
	if err != nil {
		t.Fatal(err)
	}
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
