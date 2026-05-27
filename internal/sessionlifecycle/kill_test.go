package sessionlifecycle_test

import (
	"errors"
	"testing"

	"github.com/dhruvsaxena1998/cleo/internal/sessionlifecycle"
	"github.com/dhruvsaxena1998/cleo/internal/state"
)

func TestKillExistingRunningSessionCallsTmuxKillAndDeletesSession(t *testing.T) {
	root := t.TempDir()
	target := mkdirProjectDir(t, "myapp")
	p := pathsFromRoot(root)
	projectStore, lifecycle, fake := newLifecycleWithProject(t, p, target)
	stateStore := stateFromLifecycle(t, p)
	sid := seedSession(t, stateStore, projectStore, "myapp", state.Running, "test-session")

	result, err := lifecycle.Kill(sid)
	if err != nil {
		t.Fatal(err)
	}
	if result.SessionID != sid {
		t.Fatalf("result SessionID = %q, want %q", result.SessionID, sid)
	}
	if result.Warning != nil {
		t.Fatalf("unexpected warning: %v", result.Warning)
	}
	if len(fake.killed) != 1 || fake.killed[0] != sid {
		t.Fatalf("tmux killed = %v, want [%q]", fake.killed, sid)
	}
	if _, err := stateStore.Get(sid); !errors.Is(err, state.ErrSessionNotFound) {
		t.Fatalf("session should be deleted after kill, got err=%v", err)
	}
}

func TestKillExistingFinishedSessionDeletesRecord(t *testing.T) {
	root := t.TempDir()
	target := mkdirProjectDir(t, "myapp")
	p := pathsFromRoot(root)
	projectStore, lifecycle, _ := newLifecycleWithProject(t, p, target)
	stateStore := stateFromLifecycle(t, p)
	sid := seedSession(t, stateStore, projectStore, "myapp", state.Completed, "test-session")

	result, err := lifecycle.Kill(sid)
	if err != nil {
		t.Fatal(err)
	}
	if result.Warning != nil {
		t.Fatalf("unexpected warning: %v", result.Warning)
	}
	// tmux kill may be called (harmless for finished sessions)
	if _, err := stateStore.Get(sid); !errors.Is(err, state.ErrSessionNotFound) {
		t.Fatalf("session should be deleted after kill, got err=%v", err)
	}
}

func TestKillUnknownSessionReturnsErrSessionNotFound(t *testing.T) {
	root := t.TempDir()
	p := pathsFromRoot(root)
	_, lifecycle, _ := newLifecycleWithProject(t, p, mkdirProjectDir(t, "myapp"))

	_, err := lifecycle.Kill("cleo-nonexistent-project-agent-session")
	if err == nil {
		t.Fatal("expected error for unknown session")
	}
	if !errors.Is(err, sessionlifecycle.ErrSessionNotFound) {
		t.Fatalf("error = %v, want ErrSessionNotFound", err)
	}
}

func TestKillTmuxFailureReturnsWarningButStillDeletesSession(t *testing.T) {
	root := t.TempDir()
	target := mkdirProjectDir(t, "myapp")
	p := pathsFromRoot(root)
	projectStore, lifecycle, fake := newLifecycleWithProject(t, p, target)
	stateStore := stateFromLifecycle(t, p)
	sid := seedSession(t, stateStore, projectStore, "myapp", state.Running, "test-session")
	fake.killErr = errors.New("tmux refuses")

	result, err := lifecycle.Kill(sid)
	if err != nil {
		t.Fatal(err)
	}
	if result.Warning == nil {
		t.Fatal("expected warning for tmux failure")
	}
	if !errors.Is(result.Warning, fake.killErr) {
		t.Fatalf("warning error = %v, want %v", result.Warning, fake.killErr)
	}
	if _, err := stateStore.Get(sid); !errors.Is(err, state.ErrSessionNotFound) {
		t.Fatalf("session should still be deleted after tmux kill failure, got err=%v", err)
	}
}
