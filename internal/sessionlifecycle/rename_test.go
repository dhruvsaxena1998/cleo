package sessionlifecycle_test

import (
	"errors"
	"testing"

	"github.com/dhruvsaxena1998/cleo/internal/sessionlifecycle"
	"github.com/dhruvsaxena1998/cleo/internal/state"
)

func TestRenameSlugifiesNewName(t *testing.T) {
	h := newTestHarness(t)
	pid := h.addProject(t, "myapp")
	sid := h.seedSession(t, pid, state.Running, "old-name")

	result, err := h.Lifecycle.Rename(sid, "New Display Name!")
	if err != nil {
		t.Fatal(err)
	}
	if result.SessionID != sid {
		t.Fatalf("SessionID = %q, want %q", result.SessionID, sid)
	}
	if result.OldName != "old-name" {
		t.Fatalf("OldName = %q, want %q", result.OldName, "old-name")
	}
	if result.NewName != "new-display-name" {
		t.Fatalf("NewName = %q, want %q", result.NewName, "new-display-name")
	}

	// Verify it's persisted.
	stored, err := h.State.Get(sid)
	if err != nil {
		t.Fatal(err)
	}
	if stored.Name != "new-display-name" {
		t.Fatalf("stored Name = %q, want %q", stored.Name, "new-display-name")
	}
	if stored.ID != sid {
		t.Fatalf("stored ID changed to %q, should remain %q", stored.ID, sid)
	}
}

func TestRenameReturnsOldAndNewNames(t *testing.T) {
	h := newTestHarness(t)
	pid := h.addProject(t, "myapp")
	sid := h.seedSession(t, pid, state.Running, "before")

	result, err := h.Lifecycle.Rename(sid, "after")
	if err != nil {
		t.Fatal(err)
	}
	if result.OldName != "before" || result.NewName != "after" {
		t.Fatalf("expected before→after, got %q→%q", result.OldName, result.NewName)
	}
}

func TestRenameUnknownSessionReturnsErrSessionNotFound(t *testing.T) {
	h := newTestHarness(t)
	h.addProject(t, "myapp")

	_, err := h.Lifecycle.Rename("cleo-nonexistent-project-agent-session", "new-name")
	if err == nil {
		t.Fatal("expected error for unknown session")
	}
	if !errors.Is(err, sessionlifecycle.ErrSessionNotFound) {
		t.Fatalf("error = %v, want ErrSessionNotFound", err)
	}
}

func TestRenameDoesNotChangeSessionID(t *testing.T) {
	h := newTestHarness(t)
	pid := h.addProject(t, "myapp")
	sid := h.seedSession(t, pid, state.Running, "original")

	_, err := h.Lifecycle.Rename(sid, "updated-name")
	if err != nil {
		t.Fatal(err)
	}

	stored, err := h.State.Get(sid)
	if err != nil {
		t.Fatal(err)
	}
	if stored.ID != sid {
		t.Fatalf("session ID changed from %q to %q", sid, stored.ID)
	}
}
