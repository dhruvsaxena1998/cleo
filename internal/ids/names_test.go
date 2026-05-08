package ids

import (
	"strings"
	"testing"
)

func TestRandomNameReturnsDockerStyleSlug(t *testing.T) {
	got := RandomName(nil)
	parts := strings.Split(got, "-")
	if len(parts) != 2 {
		t.Fatalf("expected adjective-name slug, got %q", got)
	}
	if got != Slugify(got) {
		t.Fatalf("expected slug-safe name, got %q", got)
	}
}

func TestRandomNameAvoidsExistingNames(t *testing.T) {
	existing := map[string]bool{}
	for i := 0; i < len(nameAdjectives)*len(nameNouns)-1; i++ {
		existing[nameAt(i)] = true
	}
	want := nameAt(len(nameAdjectives)*len(nameNouns) - 1)
	if got := RandomName(existing); got != want {
		t.Fatalf("expected only available name %q, got %q", want, got)
	}
}

func TestRandomNameDedupesWhenPoolExhausted(t *testing.T) {
	existing := map[string]bool{}
	for i := 0; i < len(nameAdjectives)*len(nameNouns); i++ {
		existing[nameAt(i)] = true
	}
	got := RandomName(existing)
	if !strings.Contains(got, "-2") {
		t.Fatalf("expected deduped exhausted-pool name, got %q", got)
	}
}
