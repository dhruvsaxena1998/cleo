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
