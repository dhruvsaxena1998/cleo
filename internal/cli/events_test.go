package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/dhruvsaxena1998/cleo/internal/events"
)

// safeBuffer is a bytes.Buffer with a mutex around writes/reads, used by the
// follow test where the cobra goroutine writes while the main goroutine polls.
// bytes.Buffer is documented as not safe for concurrent use; without the
// mutex, `go test -race` reports a data race.
type safeBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (s *safeBuffer) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.buf.Write(p)
}

func (s *safeBuffer) String() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.buf.String()
}

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
	var buf safeBuffer
	cmd.SetOut(&buf)

	done := make(chan error, 1)
	go func() { done <- cmd.Execute() }()

	// Stop the follow goroutine on cleanup by deleting the log file —
	// tailLoop exits cleanly when its path disappears (the same exit path
	// triggered in production by `cleo prune` archiving the session).
	t.Cleanup(func() {
		_ = os.Remove(logPath)
		select {
		case <-done:
		case <-time.After(2 * time.Second):
			t.Log("follow goroutine did not exit within 2s of file removal")
		}
	})

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
}

func TestEventsCmdResolvesSubstringAndArchive(t *testing.T) {
	c, _ := testCtxWithRoot(t)

	// Create one active session log
	activeID := "cleo-myapp-claude-active-thing"
	activeLog := events.NewLog(c.Paths.EventsLog(activeID))
	if err := activeLog.Append(events.Entry{At: time.Now().UTC(), Type: "session_start"}); err != nil {
		t.Fatalf("active seed: %v", err)
	}

	// Create one archived log via Archive helper
	archiveID := "cleo-myapp-claude-archived-thing"
	archiveSrc := c.Paths.EventsLog(archiveID)
	archiveLog := events.NewLog(archiveSrc)
	if err := archiveLog.Append(events.Entry{At: time.Now().UTC(), Type: "session_end"}); err != nil {
		t.Fatalf("archive seed: %v", err)
	}
	if err := events.Archive(archiveSrc, c.Paths.ArchiveDir()); err != nil {
		t.Fatalf("archive: %v", err)
	}

	// Substring match across both
	cmd := NewRootCmd(func(*Ctx) error { return nil })
	cmd.SetArgs([]string{"events", "active-thing", "-n", "10"})
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("active substring: %v", err)
	}
	if !strings.Contains(buf.String(), "session_start") {
		t.Errorf("active substring output: %q", buf.String())
	}

	// Archive substring match
	buf.Reset()
	cmd.SetArgs([]string{"events", "archived-thing", "-n", "10"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("archive substring: %v", err)
	}
	if !strings.Contains(buf.String(), "session_end") {
		t.Errorf("archive substring output: %q", buf.String())
	}

	// Ambiguous substring matches both → error
	buf.Reset()
	cmd.SetArgs([]string{"events", "myapp-claude", "-n", "10"})
	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "ambiguous") {
		t.Errorf("expected ambiguous error, got %v / output %q", err, buf.String())
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
