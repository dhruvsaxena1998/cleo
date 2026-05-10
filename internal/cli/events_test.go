package cli

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/dhruvsaxena1998/cleo/internal/events"
)

func TestEventsCmdPrintsActiveSession(t *testing.T) {
	c, _ := testCtxWithRoot(t)

	// Seed an event log
	log := events.NewLog(c.Paths.EventsLog("cleo-foo-claude-bar"))
	if err := log.Append(events.Entry{At: time.Now().UTC(), Type: "session_start"}); err != nil {
		t.Fatalf("seed: %v", err)
	}

	cmd := NewRootCmd(func(*Ctx) error { return nil })
	cmd.SetArgs([]string{"events", "cleo-foo-claude-bar", "-n", "10"})
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if !strings.Contains(buf.String(), "session_start") {
		t.Errorf("output missing session_start: %q", buf.String())
	}

	// JSON mode passes through raw lines
	buf.Reset()
	cmd.SetArgs([]string{"events", "cleo-foo-claude-bar", "--json"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute json: %v", err)
	}
	var entry events.Entry
	if err := json.Unmarshal([]byte(strings.TrimSpace(buf.String())), &entry); err != nil {
		t.Errorf("json output not valid: %q (%v)", buf.String(), err)
	}
}

func TestEventsCmdFollowEmitsAppendedLines(t *testing.T) {
	c, _ := testCtxWithRoot(t)
	logPath := c.Paths.EventsLog("cleo-foo-claude-bar")
	log := events.NewLog(logPath)
	if err := log.Append(events.Entry{At: time.Now().UTC(), Type: "session_start"}); err != nil {
		t.Fatalf("seed: %v", err)
	}

	cmd := NewRootCmd(func(*Ctx) error { return nil })
	cmd.SetArgs([]string{"events", "cleo-foo-claude-bar", "-f", "--json"})
	var buf bytes.Buffer
	cmd.SetOut(&buf)

	done := make(chan error, 1)
	go func() { done <- cmd.Execute() }()

	// Wait for initial dump
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) && !strings.Contains(buf.String(), "session_start") {
		time.Sleep(50 * time.Millisecond)
	}
	if !strings.Contains(buf.String(), "session_start") {
		t.Fatalf("initial dump missing: %q", buf.String())
	}

	// Append a second event, expect it to appear within ~1s
	if err := log.Append(events.Entry{At: time.Now().UTC(), Type: "post_tool_use"}); err != nil {
		t.Fatalf("append: %v", err)
	}
	deadline = time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) && !strings.Contains(buf.String(), "post_tool_use") {
		time.Sleep(100 * time.Millisecond)
	}
	if !strings.Contains(buf.String(), "post_tool_use") {
		t.Fatalf("appended event not seen: %q", buf.String())
	}

	// Cancel the follow by sending SIGINT — for the test we just stop reading.
	// In production a user hits Ctrl-C; here we let the test goroutine leak.
	_ = done
}

// testCtxWithRoot creates a Ctx rooted at a tempdir and returns (ctx, root).
func testCtxWithRoot(t *testing.T) (*Ctx, string) {
	t.Helper()
	root := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", root)
	c, err := NewCtxWithRoot(filepath.Join(root, "cleo"))
	if err != nil {
		t.Fatalf("ctx: %v", err)
	}
	return c, filepath.Join(root, "cleo")
}
