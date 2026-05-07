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
