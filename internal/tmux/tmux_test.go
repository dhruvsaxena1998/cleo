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
	want := []string{"capture-pane", "-e", "-p", "-S", "-50", "-t", "cleo-foo:."}
	if !equalStrings(args, want) {
		t.Errorf("argv: want %v, got %v", want, args)
	}

	// Default fallback when lines <= 0
	args = capturePaneArgs("cleo-bar", 0)
	if args[4] != "-30" {
		t.Errorf("default lines: want -30, got %s", args[4])
	}
}

// attachArgs is pure, so the switch-client-vs-attach-session decision is a
// table: inside an existing tmux client we switch the client (so attach does
// not nest a tmux inside the current pane); otherwise we attach the session.
// The target session id must always appear in the argv.
func TestAttachArgs(t *testing.T) {
	tests := []struct {
		name       string
		sessionID  string
		insideTmux bool
		want       []string
	}{
		{
			name:       "inside tmux switches the client",
			sessionID:  "cleo-myapp-claude-1",
			insideTmux: true,
			want:       []string{"switch-client", "-t", "cleo-myapp-claude-1"},
		},
		{
			name:       "outside tmux attaches the session",
			sessionID:  "cleo-myapp-claude-1",
			insideTmux: false,
			want:       []string{"attach-session", "-t", "cleo-myapp-claude-1"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := attachArgs(tt.sessionID, tt.insideTmux)
			if !equalStrings(got, tt.want) {
				t.Errorf("attachArgs(%q, %v) = %v, want %v", tt.sessionID, tt.insideTmux, got, tt.want)
			}
		})
	}
}

// AttachCmd must build (not run) a socket-aware command and pick the subcommand
// from $TMUX. Asserting on cmd.Args proves the -L socket flag is honored — the
// latent bug the seam closes — and that the inside/outside decision is wired in.
func TestAttachCmdHonorsSocketAndInsideTmuxDecision(t *testing.T) {
	c := NewClient("cleo-sock")

	t.Setenv("TMUX", "/tmp/tmux-1000/default,9999,0")
	got := c.AttachCmd("cleo-myapp-claude-1")
	want := []string{"tmux", "-L", "cleo-sock", "switch-client", "-t", "cleo-myapp-claude-1"}
	if !equalStrings(got.Args, want) {
		t.Errorf("inside tmux: argv = %v, want %v", got.Args, want)
	}

	t.Setenv("TMUX", "")
	got = c.AttachCmd("cleo-myapp-claude-1")
	want = []string{"tmux", "-L", "cleo-sock", "attach-session", "-t", "cleo-myapp-claude-1"}
	if !equalStrings(got.Args, want) {
		t.Errorf("outside tmux: argv = %v, want %v", got.Args, want)
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

func TestSendKeys(t *testing.T) {
	c := newTestClient(t)
	// cat reads from stdin and echoes — perfect for verifying send-keys
	_ = c.NewSession(NewSessionOpts{Name: "cleo-send-test", Cwd: "/tmp", Cmd: "cat"})
	time.Sleep(150 * time.Millisecond)

	if err := c.SendKeys("cleo-send-test", "hello from cleo"); err != nil {
		t.Fatal(err)
	}
	// Give cat a moment to echo
	time.Sleep(150 * time.Millisecond)

	out, err := c.CapturePane("cleo-send-test", 50)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "hello from cleo") {
		t.Errorf("missing sent text in pane: %q", out)
	}
}

func TestSendKeysMultiLine(t *testing.T) {
	c := newTestClient(t)
	_ = c.NewSession(NewSessionOpts{Name: "cleo-send-ml", Cwd: "/tmp", Cmd: "cat"})
	time.Sleep(150 * time.Millisecond)

	text := "line one\nline two"
	if err := c.SendKeys("cleo-send-ml", text); err != nil {
		t.Fatal(err)
	}
	time.Sleep(150 * time.Millisecond)

	out, err := c.CapturePane("cleo-send-ml", 50)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "line one") || !strings.Contains(out, "line two") {
		t.Errorf("missing multi-line text in pane: %q", out)
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
