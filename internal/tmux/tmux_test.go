package tmux

import (
	"fmt"
	"math/rand"
	"strings"
	"testing"
	"time"
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

func TestCapturePane(t *testing.T) {
	c := newTestClient(t)
	_ = c.NewSession(NewSessionOpts{Name: "cleo-cap-1", Cwd: "/tmp", Cmd: "echo HELLO_WORLD; sleep 60"})
	// give shell a moment
	time.Sleep(150 * time.Millisecond)
	out, err := c.CapturePane("cleo-cap-1", 50)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "HELLO_WORLD") {
		t.Errorf("missing token in capture: %q", out)
	}
}

func TestCapturePaneArgsIncludeScrollbackFlag(t *testing.T) {
	args := capturePaneArgs("cleo-foo", 50)
	want := []string{"capture-pane", "-p", "-S", "-50", "-t", "cleo-foo:."}
	if !equalStrings(args, want) {
		t.Errorf("argv: want %v, got %v", want, args)
	}

	// Default fallback when lines <= 0
	args = capturePaneArgs("cleo-bar", 0)
	if args[3] != "-30" {
		t.Errorf("default lines: want -30, got %s", args[3])
	}
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestRenameSession(t *testing.T) {
	c := newTestClient(t)
	_ = c.NewSession(NewSessionOpts{Name: "old", Cwd: "/tmp", Cmd: "sleep 60"})
	if err := c.RenameSession("old", "new"); err != nil {
		t.Fatal(err)
	}
	ok, _ := c.HasSession("new")
	if !ok {
		t.Errorf("expected new")
	}
}

func TestNewSession_SetsAllowPassthrough(t *testing.T) {
	c := newTestClient(t)
	name := "cleo-pt-test-1"
	if err := c.NewSession(NewSessionOpts{Name: name, Cwd: "/tmp", Cmd: "sleep 60"}); err != nil {
		t.Fatal(err)
	}
	out, err := c.cmd("show-options", "-pt", name, "allow-passthrough").Output()
	if err != nil {
		t.Skipf("tmux version does not support allow-passthrough: %v", err)
	}
	if !strings.Contains(string(out), "allow-passthrough on") {
		t.Errorf("expected allow-passthrough on, got: %q", string(out))
	}
}
