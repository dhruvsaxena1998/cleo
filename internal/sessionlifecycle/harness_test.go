package sessionlifecycle_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/dhruvsaxena1998/cleo/internal/events"
	"github.com/dhruvsaxena1998/cleo/internal/focus"
	"github.com/dhruvsaxena1998/cleo/internal/ids"
	"github.com/dhruvsaxena1998/cleo/internal/paths"
	"github.com/dhruvsaxena1998/cleo/internal/projects"
	"github.com/dhruvsaxena1998/cleo/internal/sessionlifecycle"
	"github.com/dhruvsaxena1998/cleo/internal/state"
)

// testHarness holds all dependencies for a lifecycle test.
type testHarness struct {
	Paths     paths.Paths
	Projects  *projects.Store
	State     *state.Store
	Tmux      *fakeTmux
	Focus     *focus.Store
	Lifecycle *sessionlifecycle.Lifecycle
}

func newTestHarness(t *testing.T) *testHarness {
	t.Helper()
	root := t.TempDir()
	p := paths.NewWithRoot(root)
	fake := &fakeTmux{verifySession: true, hasSession: true}
	projectStore := projects.NewStore(p.ProjectsFile())
	focusStore := focus.NewStore(p.FocusFile())
	l := sessionlifecycle.New(sessionlifecycle.Options{
		Config:   testConfig(),
		Projects: projectStore,
		State:    state.NewStore(p.StateFile(), p.StateLock()),
		Tmux:     fake,
		Paths:    p,
		Focus:    focusStore,
	})
	return &testHarness{
		Paths:     p,
		Projects:  projectStore,
		State:     state.NewStore(p.StateFile(), p.StateLock()),
		Tmux:      fake,
		Focus:     focusStore,
		Lifecycle: l,
	}
}

// addProject registers a project and returns its ID.
func (h *testHarness) addProject(t *testing.T, name string) string {
	t.Helper()
	target := filepath.Join(t.TempDir(), name)
	if err := os.MkdirAll(target, 0o755); err != nil {
		t.Fatal(err)
	}
	proj, err := h.Projects.Add(target)
	if err != nil {
		t.Fatal(err)
	}
	return proj.ID
}

// seedSession writes a session into the state store and returns its ID.
// name is a unique suffix used to construct the session ID and display name.
func (h *testHarness) seedSession(t *testing.T, projectID string, st state.State, name string) string {
	t.Helper()
	sid := ids.MakeSessionID(projectID, "claude", name)
	if err := h.State.Put(state.Session{
		ID:        sid,
		ProjectID: projectID,
		Agent:     "claude",
		Name:      name,
		State:     st,
	}); err != nil {
		t.Fatal(err)
	}
	return sid
}

// writeEventLog creates an event log entry for the given session.
func (h *testHarness) writeEventLog(t *testing.T, sid string) {
	t.Helper()
	l := events.NewLog(h.Paths.EventsLog(sid))
	if err := l.Append(events.Entry{Type: "test"}); err != nil {
		t.Fatal(err)
	}
}

// assertEventLogExists fails if the active event log for sid is missing.
func (h *testHarness) assertEventLogExists(t *testing.T, sid string) {
	t.Helper()
	if _, err := os.Stat(h.Paths.EventsLog(sid)); os.IsNotExist(err) {
		t.Fatalf("expected event log for %s to exist at %s", sid, h.Paths.EventsLog(sid))
	}
}

// assertEventLogArchived fails if the gzipped archived event log is missing.
func (h *testHarness) assertEventLogArchived(t *testing.T, sid string) {
	t.Helper()
	archivePath := filepath.Join(h.Paths.ArchiveDir(), sid+".jsonl.gz")
	if _, err := os.Stat(archivePath); os.IsNotExist(err) {
		t.Fatalf("expected archived event log for %s at %s", sid, archivePath)
	}
}

// assertEventLogDeleted fails if the active event log still exists.
func (h *testHarness) assertEventLogDeleted(t *testing.T, sid string) {
	t.Helper()
	if _, err := os.Stat(h.Paths.EventsLog(sid)); !os.IsNotExist(err) {
		t.Fatalf("expected event log for %s to be deleted at %s", sid, h.Paths.EventsLog(sid))
	}
}

// pathsFromRoot creates a paths.Paths rooted at root (for use with t.TempDir()).
func pathsFromRoot(root string) paths.Paths {
	return paths.NewWithRoot(root)
}

// stateFromLifecycle creates a new *state.Store for the same backing file used by
// the lifecycle. Tests that need direct state assertions use this to avoid locking
// conflicts with the lifecycle's internal state store.
func stateFromLifecycle(t *testing.T, p paths.Paths) *state.Store {
	t.Helper()
	return state.NewStore(p.StateFile(), p.StateLock())
}

// newLifecycleWithProject creates a lifecycle with a registered project and returns
// the project store, lifecycle, and the fake tmux. The project is registered at a temp directory.
func newLifecycleWithProject(t *testing.T, p paths.Paths, projectPath string) (*projects.Store, *sessionlifecycle.Lifecycle, *fakeTmux) {
	t.Helper()
	projectStore := projects.NewStore(p.ProjectsFile())
	if _, err := projectStore.Add(projectPath); err != nil {
		t.Fatal(err)
	}
	fake := &fakeTmux{verifySession: true, hasSession: true}
	l := sessionlifecycle.New(sessionlifecycle.Options{
		Config:   testConfig(),
		Projects: projectStore,
		State:    state.NewStore(p.StateFile(), p.StateLock()),
		Tmux:     fake,
		Paths:    p,
	})
	return projectStore, l, fake
}

// seedSession writes a session into the state store identified by projectID+agent+name.
func seedSession(t *testing.T, s *state.Store, proj *projects.Store, projectID string, st state.State, name string) string {
	t.Helper()
	sid := ids.MakeSessionID(projectID, "claude", name)
	if err := s.Put(state.Session{
		ID:        sid,
		ProjectID: projectID,
		Agent:     "claude",
		Name:      name,
		State:     st,
	}); err != nil {
		t.Fatal(err)
	}
	return sid
}

// writeEventLog creates an event log entry for the given session under paths.Paths.
func writeEventLog(t *testing.T, p paths.Paths, sid string) {
	t.Helper()
	l := events.NewLog(p.EventsLog(sid))
	if err := l.Append(events.Entry{Type: "test"}); err != nil {
		t.Fatal(err)
	}
}
