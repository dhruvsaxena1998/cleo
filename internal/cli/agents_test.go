package cli

import (
	"strings"
	"testing"
)

func TestParseAgentsFlag_SingleAgent(t *testing.T) {
	got, err := parseAgentsFlag("claude")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 1 || got[0] != "claude" {
		t.Errorf("got %v, want [claude]", got)
	}
}

func TestParseAgentsFlag_EmptyErrors(t *testing.T) {
	_, err := parseAgentsFlag("")
	if err == nil {
		t.Fatal("expected error for empty input, got nil")
	}
}

func TestParseAgentsFlag_UnknownErrors(t *testing.T) {
	_, err := parseAgentsFlag("claude,bogus")
	if err == nil {
		t.Fatal("expected error for unknown agent, got nil")
	}
	if !strings.Contains(err.Error(), "bogus") {
		t.Errorf("error %q should name the offending agent", err.Error())
	}
}

func TestParseAgentsFlag_DuplicatesCollapsed(t *testing.T) {
	got, err := parseAgentsFlag("claude,codex,claude,CODEX")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []string{"claude", "codex"}
	if !equalStrs(got, want) {
		t.Errorf("got %v, want %v (duplicates should collapse, preserving first occurrence)", got, want)
	}
}

func TestParseAgentsFlag_NormalizesCase(t *testing.T) {
	got, err := parseAgentsFlag("Claude,CODEX,OpenCode")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []string{"claude", "codex", "opencode"}
	if !equalStrs(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestParseAgentsFlag_CommaSeparated(t *testing.T) {
	got, err := parseAgentsFlag("claude,codex")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []string{"claude", "codex"}
	if !equalStrs(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func equalStrs(a, b []string) bool {
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
