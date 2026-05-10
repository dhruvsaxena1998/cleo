package events

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestAppendAndTail(t *testing.T) {
	dir := t.TempDir()
	log := NewLog(filepath.Join(dir, "x.jsonl"))
	for i := 0; i < 3; i++ {
		if err := log.Append(Entry{
			At:   time.Now(),
			Type: "PreToolUse",
			Tool: "Bash",
		}); err != nil {
			t.Fatal(err)
		}
	}
	got, err := log.Tail(10)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 3 {
		t.Errorf("got %d", len(got))
	}
}

func TestTailLimitsToN(t *testing.T) {
	dir := t.TempDir()
	log := NewLog(filepath.Join(dir, "x.jsonl"))
	for i := 0; i < 10; i++ {
		_ = log.Append(Entry{Type: "x"})
	}
	got, _ := log.Tail(3)
	if len(got) != 3 {
		t.Errorf("got %d", len(got))
	}
}

func TestReadFiltered(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "x.jsonl")
	log := NewLog(path)
	now := time.Now().UTC()
	older := now.Add(-2 * time.Hour)
	for _, e := range []Entry{
		{At: older, Type: "session_start"},
		{At: older.Add(time.Hour), Type: "notification"},
		{At: now, Type: "post_tool_use"},
	} {
		if err := log.Append(e); err != nil {
			t.Fatalf("append: %v", err)
		}
	}

	// No filters
	got, err := log.ReadFiltered(ReadOpts{})
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if len(got) != 3 {
		t.Errorf("len no-filter: want 3, got %d", len(got))
	}

	// Type filter
	got, _ = log.ReadFiltered(ReadOpts{Type: "notification"})
	if len(got) != 1 || got[0].Type != "notification" {
		t.Errorf("type filter: %+v", got)
	}

	// Since filter (last 30 minutes — only post_tool_use is newer than that)
	got, _ = log.ReadFiltered(ReadOpts{Since: now.Add(-30 * time.Minute)})
	if len(got) != 1 || got[0].Type != "post_tool_use" {
		t.Errorf("since filter: %+v", got)
	}

	// Limit
	got, _ = log.ReadFiltered(ReadOpts{Limit: 2})
	if len(got) != 2 {
		t.Errorf("limit: want 2, got %d", len(got))
	}
	// Limit returns most recent
	if got[0].Type != "notification" || got[1].Type != "post_tool_use" {
		t.Errorf("limit ordering: %+v", got)
	}
}

func TestArchive(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "x.jsonl")
	log := NewLog(src)
	_ = log.Append(Entry{Type: "x"})
	archDir := filepath.Join(dir, "archive")
	if err := Archive(src, archDir); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(src); !os.IsNotExist(err) {
		t.Errorf("src still exists")
	}
	matches, _ := filepath.Glob(filepath.Join(archDir, "x.jsonl.gz"))
	if len(matches) != 1 {
		t.Errorf("archive missing")
	}
}
