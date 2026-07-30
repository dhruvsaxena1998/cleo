package sessionlifecycle_test

import (
	"testing"

	"github.com/dhruvsaxena1998/cleo/internal/config"
	"github.com/dhruvsaxena1998/cleo/internal/paths"
	"github.com/dhruvsaxena1998/cleo/internal/projects"
	"github.com/dhruvsaxena1998/cleo/internal/sessionlifecycle"
	"github.com/dhruvsaxena1998/cleo/internal/state"
	"github.com/dhruvsaxena1998/cleo/internal/tmux"
)

// newLabelLifecycle wires a lifecycle around cfg with one registered project,
// returning it with the project ID and the fake tmux that records the labels.
func newLabelLifecycle(t *testing.T, cfg config.Config) (*sessionlifecycle.Lifecycle, string, *fakeTmux) {
	t.Helper()
	p := paths.NewWithRoot(t.TempDir())
	projectStore := projects.NewStore(p.ProjectsFile())
	registered, err := projectStore.Add(mkdirProjectDir(t, "pickup-api"))
	if err != nil {
		t.Fatal(err)
	}
	fake := &fakeTmux{}
	l := sessionlifecycle.New(sessionlifecycle.Options{
		Config:   cfg,
		Projects: projectStore,
		State:    state.NewStore(p.StateFile(), p.StateLock()),
		Tmux:     fake,
		Paths:    p,
	})
	return l, registered.ID, fake
}

func labelTestConfig() config.Config {
	cfg := testConfig()
	cfg.Agents["claude"] = config.Agent{Command: "sh", Color: "#CC785C"}
	return cfg
}

func TestCreateLabelsTheTmuxSessionWithProjectAgentAndName(t *testing.T) {
	lifecycle, projectID, fake := newLabelLifecycle(t, labelTestConfig())

	result, err := lifecycle.Create(sessionlifecycle.CreateInput{
		Agent:     "claude",
		Name:      "lucid-turing",
		ProjectID: projectID,
	})
	if err != nil {
		t.Fatal(err)
	}

	if len(fake.labels) != 1 {
		t.Fatalf("labels applied = %d, want 1", len(fake.labels))
	}
	want := tmux.SessionLabel{
		Session: result.Session.ID,
		Project: projectID,
		Agent:   "claude",
		Name:    "lucid-turing",
		Color:   "#CC785C",
	}
	if fake.labels[0] != want {
		t.Fatalf("label = %#v, want %#v", fake.labels[0], want)
	}
}

func TestAttachRelabelsTheSessionSoAgentWindowRenamesAreRepaired(t *testing.T) {
	lifecycle, projectID, fake := newLabelLifecycle(t, labelTestConfig())
	created, err := lifecycle.Create(sessionlifecycle.CreateInput{
		Agent:     "claude",
		Name:      "lucid-turing",
		ProjectID: projectID,
	})
	if err != nil {
		t.Fatal(err)
	}
	fake.labels = nil

	plan, err := lifecycle.Attach(created.Session.ID)
	if err != nil {
		t.Fatal(err)
	}
	if plan.Action != sessionlifecycle.AttachReady {
		t.Fatalf("attach action = %v, want AttachReady", plan.Action)
	}
	if len(fake.labels) != 1 || fake.labels[0].Session != created.Session.ID {
		t.Fatalf("attach should re-apply the label, got %#v", fake.labels)
	}
}

func TestAttachOnUnattachableSessionSkipsTheLabel(t *testing.T) {
	h := newTestHarness(t)
	pid := h.addProject(t, "myapp")
	sid := h.seedSession(t, pid, state.Dead, "dead-session")

	if _, err := h.Lifecycle.Attach(sid); err != nil {
		t.Fatal(err)
	}
	if len(h.Tmux.labels) != 0 {
		t.Fatalf("blocked attach should not touch tmux options, got %#v", h.Tmux.labels)
	}
}

func TestStatusLineOffLeavesTmuxDisplayOptionsAlone(t *testing.T) {
	cfg := labelTestConfig()
	cfg.Tmux.StatusLine = config.StatusLineOff
	lifecycle, projectID, fake := newLabelLifecycle(t, cfg)

	created, err := lifecycle.Create(sessionlifecycle.CreateInput{
		Agent:     "claude",
		Name:      "lucid-turing",
		ProjectID: projectID,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := lifecycle.Attach(created.Session.ID); err != nil {
		t.Fatal(err)
	}

	if len(fake.labels) != 0 {
		t.Fatalf("status_line = off should apply no labels, got %#v", fake.labels)
	}
}

func TestLabelSurvivesTmuxFailureWithoutFailingCreate(t *testing.T) {
	lifecycle, projectID, fake := newLabelLifecycle(t, labelTestConfig())
	fake.labelErr = errTmuxLabel

	if _, err := lifecycle.Create(sessionlifecycle.CreateInput{
		Agent:     "claude",
		Name:      "lucid-turing",
		ProjectID: projectID,
	}); err != nil {
		t.Fatalf("a failed status option must not fail the spawn: %v", err)
	}
}

func TestRenameRefreshesTheLabelWithTheNewName(t *testing.T) {
	lifecycle, projectID, fake := newLabelLifecycle(t, labelTestConfig())
	created, err := lifecycle.Create(sessionlifecycle.CreateInput{
		Agent:     "claude",
		Name:      "lucid-turing",
		ProjectID: projectID,
	})
	if err != nil {
		t.Fatal(err)
	}
	fake.labels = nil

	if _, err := lifecycle.Rename(created.Session.ID, "Nimble Hopper"); err != nil {
		t.Fatal(err)
	}

	if len(fake.labels) != 1 {
		t.Fatalf("labels applied = %d, want 1", len(fake.labels))
	}
	if got := fake.labels[0]; got.Name != "nimble-hopper" || got.Session != created.Session.ID {
		t.Fatalf("label = %#v, want the slugified new name on the same session", got)
	}
}
