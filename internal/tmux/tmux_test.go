package tmux

import (
	"fmt"
	"math/rand"
	"strings"
	"testing"
)

func newTestClient(t *testing.T) *Client {
	t.Helper()
	if !Available() {
		t.Skip("tmux not installed")
	}
	socket := fmt.Sprintf("cleo-test-%d", rand.Int63())
	c := NewClient(socket)
	t.Cleanup(func() { _ = c.KillServer() })
	return c
}

func TestNewSessionAndHas(t *testing.T) {
	c := newTestClient(t)
	if err := c.NewSession(NewSessionOpts{Name: "cleo-foo-claude-1", Cwd: "/tmp", Cmd: "sleep 60", Env: nil}); err != nil {
		t.Fatal(err)
	}
	ok, err := c.HasSession("cleo-foo-claude-1")
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Errorf("expected has-session true")
	}
}

func TestLsWithPrefix(t *testing.T) {
	c := newTestClient(t)
	_ = c.NewSession(NewSessionOpts{Name: "cleo-a-claude-1", Cwd: "/tmp", Cmd: "sleep 60"})
	_ = c.NewSession(NewSessionOpts{Name: "other", Cwd: "/tmp", Cmd: "sleep 60"})
	got, err := c.LsPrefix("cleo-")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || !strings.HasPrefix(got[0], "cleo-") {
		t.Errorf("got %v", got)
	}
}

func TestKill(t *testing.T) {
	c := newTestClient(t)
	_ = c.NewSession(NewSessionOpts{Name: "cleo-x-claude-1", Cwd: "/tmp", Cmd: "sleep 60"})
	if err := c.Kill("cleo-x-claude-1"); err != nil {
		t.Fatal(err)
	}
	ok, _ := c.HasSession("cleo-x-claude-1")
	if ok {
		t.Errorf("expected gone")
	}
}
