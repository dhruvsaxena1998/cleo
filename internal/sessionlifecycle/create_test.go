package sessionlifecycle_test

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dhruvsaxena1998/cleo/internal/config"
	"github.com/dhruvsaxena1998/cleo/internal/paths"
	"github.com/dhruvsaxena1998/cleo/internal/projects"
	"github.com/dhruvsaxena1998/cleo/internal/sessionlifecycle"
	"github.com/dhruvsaxena1998/cleo/internal/state"
	"github.com/dhruvsaxena1998/cleo/internal/tmux"
)

func TestCreateForUnregisteredPathNeedsRegistrationWhenAutoRegistrationIsDisabled(t *testing.T) {
	root := t.TempDir()
	target := mkdirProjectDir(t, "myapp")

	p := paths.NewWithRoot(root)
	lifecycle := newTestLifecycle(p, &fakeTmux{})

	_, err := lifecycle.Create(sessionlifecycle.CreateInput{
		Agent:               "claude",
		Path:                target,
		AutoRegisterProject: false,
	})
	if !errors.Is(err, sessionlifecycle.ErrProjectRegistrationNeeded) {
		t.Fatalf("Create error = %v, want ErrProjectRegistrationNeeded", err)
	}

	sessions, err := state.NewStore(p.StateFile(), p.StateLock()).List()
	if err != nil {
		t.Fatal(err)
	}
	if len(sessions) != 0 {
		t.Fatalf("registration-needed create wrote sessions: %#v", sessions)
	}
}

func TestCreateForUnregisteredPathWithAutoRegistrationRegistersProjectAndCreatesSession(t *testing.T) {
	root := t.TempDir()
	target := mkdirProjectDir(t, "myapp")
	p := paths.NewWithRoot(root)
	fake := &fakeTmux{}
	lifecycle := newTestLifecycle(p, fake)

	result, err := lifecycle.Create(sessionlifecycle.CreateInput{
		Agent:               "claude",
		Name:                "Fix Auth Bug",
		Path:                target,
		AutoRegisterProject: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !result.ProjectRegistered {
		t.Fatal("expected ProjectRegistered=true")
	}
	if result.Project.ID != "myapp" || result.Project.Path != target {
		t.Fatalf("project = %#v, want myapp at %q", result.Project, target)
	}
	assertCreatedSession(t, p, fake, result, "cleo-myapp-claude-fix-auth-bug", target)
}

func TestCreateWithExplicitProjectIDUsesRegisteredProjectPath(t *testing.T) {
	root := t.TempDir()
	projectPath := mkdirProjectDir(t, "myapp")
	unregisteredPath := mkdirProjectDir(t, "other")
	p := paths.NewWithRoot(root)
	projectStore := projects.NewStore(p.ProjectsFile())
	registered, err := projectStore.Add(projectPath)
	if err != nil {
		t.Fatal(err)
	}
	fake := &fakeTmux{}
	lifecycle := newTestLifecycle(p, fake)

	result, err := lifecycle.Create(sessionlifecycle.CreateInput{
		Agent:     "claude",
		Name:      "Work Here",
		Path:      unregisteredPath,
		ProjectID: registered.ID,
	})
	if err != nil {
		t.Fatal(err)
	}

	assertCreatedSession(t, p, fake, result, "cleo-myapp-claude-work-here", projectPath)
}

func TestCreateBindsConfiguredDetachKeyWhenTmuxAdapterSupportsIt(t *testing.T) {
	root := t.TempDir()
	projectPath := mkdirProjectDir(t, "myapp")
	p := paths.NewWithRoot(root)
	projectStore := projects.NewStore(p.ProjectsFile())
	registered, err := projectStore.Add(projectPath)
	if err != nil {
		t.Fatal(err)
	}
	fake := &fakeTmux{}
	cfg := testConfig()
	cfg.Tmux.DetachKey = "C-b d"
	lifecycle := sessionlifecycle.New(sessionlifecycle.Options{
		Config:   cfg,
		Projects: projectStore,
		State:    state.NewStore(p.StateFile(), p.StateLock()),
		Tmux:     fake,
	})

	_, err = lifecycle.Create(sessionlifecycle.CreateInput{
		Agent:     "claude",
		Name:      "with-detach-key",
		ProjectID: registered.ID,
	})
	if err != nil {
		t.Fatal(err)
	}
	if fake.detachKeyBound != "C-b d" {
		t.Fatalf("detach key bound = %q, want configured key", fake.detachKeyBound)
	}
}

func TestCreateInstallsFocusHooksWhenTmuxAdapterSupportsThem(t *testing.T) {
	root := t.TempDir()
	projectPath := mkdirProjectDir(t, "myapp")
	p := paths.NewWithRoot(root)
	projectStore := projects.NewStore(p.ProjectsFile())
	registered, err := projectStore.Add(projectPath)
	if err != nil {
		t.Fatal(err)
	}
	fake := &fakeTmux{}
	lifecycle := sessionlifecycle.New(sessionlifecycle.Options{
		Config:   testConfig(),
		Projects: projectStore,
		State:    state.NewStore(p.StateFile(), p.StateLock()),
		Tmux:     fake,
		CleoBin:  "/usr/local/bin/cleo",
	})

	_, err = lifecycle.Create(sessionlifecycle.CreateInput{
		Agent:     "claude",
		Name:      "with-focus",
		ProjectID: registered.ID,
	})
	if err != nil {
		t.Fatal(err)
	}
	if fake.focusHooksInstalledWith != "/usr/local/bin/cleo" {
		t.Fatalf("focus hooks installed with %q", fake.focusHooksInstalledWith)
	}
}

func TestCreateReportsMissingAgentCommandBeforeWritingSession(t *testing.T) {
	root := t.TempDir()
	projectPath := mkdirProjectDir(t, "myapp")
	p := paths.NewWithRoot(root)
	projectStore := projects.NewStore(p.ProjectsFile())
	registered, err := projectStore.Add(projectPath)
	if err != nil {
		t.Fatal(err)
	}
	stateStore := state.NewStore(p.StateFile(), p.StateLock())
	cfg := testConfig()
	cfg.Agents["codex"] = config.Agent{Command: "definitely-not-a-real-cleo-test-command"}
	fake := &fakeTmux{}
	lifecycle := sessionlifecycle.New(sessionlifecycle.Options{
		Config:   cfg,
		Projects: projectStore,
		State:    stateStore,
		Tmux:     fake,
	})

	_, err = lifecycle.Create(sessionlifecycle.CreateInput{
		Agent:     "codex",
		Name:      "missing-command",
		ProjectID: registered.ID,
	})
	if !errors.Is(err, sessionlifecycle.ErrLaunchFailed) {
		t.Fatalf("Create error = %v, want ErrLaunchFailed", err)
	}
	if !strings.Contains(err.Error(), "agent command") || !strings.Contains(err.Error(), "not found in PATH") {
		t.Fatalf("Create error = %q, want actionable missing-command message", err.Error())
	}
	if len(fake.created) != 0 {
		t.Fatalf("tmux should not launch when command is missing: %#v", fake.created)
	}
	if sessions, err := stateStore.List(); err != nil {
		t.Fatal(err)
	} else if len(sessions) != 0 {
		t.Fatalf("missing command should not write sessions: %#v", sessions)
	}
}

func TestCreateRollsBackSessionWhenTmuxLaunchReturnsButSessionIsNotAlive(t *testing.T) {
	root := t.TempDir()
	projectPath := mkdirProjectDir(t, "myapp")
	p := paths.NewWithRoot(root)
	projectStore := projects.NewStore(p.ProjectsFile())
	registered, err := projectStore.Add(projectPath)
	if err != nil {
		t.Fatal(err)
	}
	stateStore := state.NewStore(p.StateFile(), p.StateLock())
	fake := &fakeTmux{verifySession: true, hasSession: false}
	lifecycle := sessionlifecycle.New(sessionlifecycle.Options{
		Config:   testConfig(),
		Projects: projectStore,
		State:    stateStore,
		Tmux:     fake,
	})

	_, err = lifecycle.Create(sessionlifecycle.CreateInput{
		Agent:     "claude",
		Name:      "exits-immediately",
		ProjectID: registered.ID,
	})
	if !errors.Is(err, sessionlifecycle.ErrLaunchFailed) {
		t.Fatalf("Create error = %v, want ErrLaunchFailed", err)
	}
	if _, err := stateStore.Get("cleo-myapp-claude-exits-immediately"); !errors.Is(err, state.ErrSessionNotFound) {
		t.Fatalf("exited create should roll back session, got err=%v", err)
	}
}

func TestCreateRollsBackSessionWhenTmuxLaunchFails(t *testing.T) {
	root := t.TempDir()
	projectPath := mkdirProjectDir(t, "myapp")
	p := paths.NewWithRoot(root)
	projectStore := projects.NewStore(p.ProjectsFile())
	registered, err := projectStore.Add(projectPath)
	if err != nil {
		t.Fatal(err)
	}
	stateStore := state.NewStore(p.StateFile(), p.StateLock())
	fake := &fakeTmux{onNewSession: func(tmux.NewSessionOpts) error {
		return fmt.Errorf("tmux refused session")
	}}
	lifecycle := sessionlifecycle.New(sessionlifecycle.Options{
		Config:   testConfig(),
		Projects: projectStore,
		State:    stateStore,
		Tmux:     fake,
	})

	_, err = lifecycle.Create(sessionlifecycle.CreateInput{
		Agent:     "claude",
		Name:      "will-fail",
		ProjectID: registered.ID,
	})
	if err == nil || !strings.Contains(err.Error(), "tmux refused session") {
		t.Fatalf("Create error = %v, want tmux failure", err)
	}
	if _, err := stateStore.Get("cleo-myapp-claude-will-fail"); !errors.Is(err, state.ErrSessionNotFound) {
		t.Fatalf("failed create should roll back session, got err=%v", err)
	}
}

func TestCreateWritesSpawningSessionBeforeLaunchingTmux(t *testing.T) {
	root := t.TempDir()
	projectPath := mkdirProjectDir(t, "myapp")
	p := paths.NewWithRoot(root)
	projectStore := projects.NewStore(p.ProjectsFile())
	registered, err := projectStore.Add(projectPath)
	if err != nil {
		t.Fatal(err)
	}
	stateStore := state.NewStore(p.StateFile(), p.StateLock())
	fake := &fakeTmux{}
	fake.onNewSession = func(o tmux.NewSessionOpts) error {
		stored, err := stateStore.Get(o.Name)
		if err != nil {
			t.Fatalf("session was not stored before tmux launch: %v", err)
		}
		if stored.State != state.Spawning {
			t.Fatalf("stored state during tmux launch = %s, want spawning", stored.State)
		}
		return nil
	}
	lifecycle := sessionlifecycle.New(sessionlifecycle.Options{
		Config:       testConfig(),
		Projects:     projectStore,
		State:        stateStore,
		Tmux:         fake,
		GenerateName: func(map[string]bool) string { return "state-first" },
	})

	if _, err := lifecycle.Create(sessionlifecycle.CreateInput{
		Agent:     "claude",
		ProjectID: registered.ID,
	}); err != nil {
		t.Fatal(err)
	}
}

func TestCreateWithoutRequestedNameUsesConfiguredNameGeneratorAndDedupes(t *testing.T) {
	root := t.TempDir()
	projectPath := mkdirProjectDir(t, "myapp")
	p := paths.NewWithRoot(root)
	projectStore := projects.NewStore(p.ProjectsFile())
	registered, err := projectStore.Add(projectPath)
	if err != nil {
		t.Fatal(err)
	}
	stateStore := state.NewStore(p.StateFile(), p.StateLock())
	if err := stateStore.Put(state.Session{
		ID:        "cleo-myapp-claude-brave-ada",
		ProjectID: registered.ID,
		Agent:     "claude",
		Name:      "brave-ada",
		State:     state.Running,
	}); err != nil {
		t.Fatal(err)
	}
	fake := &fakeTmux{}
	lifecycle := sessionlifecycle.New(sessionlifecycle.Options{
		Config:   testConfig(),
		Projects: projectStore,
		State:    stateStore,
		Tmux:     fake,
		GenerateName: func(existing map[string]bool) string {
			if !existing["brave-ada"] {
				t.Fatal("generator did not receive existing Session names")
			}
			return "brave-ada"
		},
	})

	result, err := lifecycle.Create(sessionlifecycle.CreateInput{
		Agent:     "claude",
		ProjectID: registered.ID,
	})
	if err != nil {
		t.Fatal(err)
	}

	assertCreatedSession(t, p, fake, result, "cleo-myapp-claude-brave-ada-2", projectPath)
}

func TestCreateDedupeRequestedNameAgainstExistingSessionsForSameProjectAndAgent(t *testing.T) {
	root := t.TempDir()
	projectPath := mkdirProjectDir(t, "myapp")
	p := paths.NewWithRoot(root)
	projectStore := projects.NewStore(p.ProjectsFile())
	registered, err := projectStore.Add(projectPath)
	if err != nil {
		t.Fatal(err)
	}
	stateStore := state.NewStore(p.StateFile(), p.StateLock())
	if err := stateStore.Put(state.Session{
		ID:        "cleo-myapp-claude-fix-auth-bug",
		ProjectID: registered.ID,
		Agent:     "claude",
		Name:      "fix-auth-bug",
		State:     state.Running,
	}); err != nil {
		t.Fatal(err)
	}
	fake := &fakeTmux{}
	lifecycle := newTestLifecycle(p, fake)

	result, err := lifecycle.Create(sessionlifecycle.CreateInput{
		Agent:     "claude",
		Name:      "Fix Auth Bug",
		ProjectID: registered.ID,
	})
	if err != nil {
		t.Fatal(err)
	}

	assertCreatedSession(t, p, fake, result, "cleo-myapp-claude-fix-auth-bug-2", projectPath)
}

func assertCreatedSession(t *testing.T, p paths.Paths, fake *fakeTmux, result sessionlifecycle.CreateResult, wantID, wantCwd string) {
	t.Helper()
	if result.Session.ID != wantID {
		t.Fatalf("session ID = %q, want %q", result.Session.ID, wantID)
	}
	if result.Session.State != state.Spawning {
		t.Fatalf("session state = %s, want spawning", result.Session.State)
	}
	if len(fake.created) != 1 || fake.created[0].Name != result.Session.ID || fake.created[0].Cwd != wantCwd {
		t.Fatalf("tmux created = %#v, want one launch for %q in %q", fake.created, result.Session.ID, wantCwd)
	}
	if fake.created[0].Cmd != "sh" {
		t.Fatalf("tmux command = %q, want sh", fake.created[0].Cmd)
	}
	if fake.created[0].Env["CLEO_SESSION_ID"] != result.Session.ID {
		t.Fatalf("tmux CLEO_SESSION_ID = %q, want %q", fake.created[0].Env["CLEO_SESSION_ID"], result.Session.ID)
	}

	stored, err := state.NewStore(p.StateFile(), p.StateLock()).Get(result.Session.ID)
	if err != nil {
		t.Fatal(err)
	}
	if stored.ID != result.Session.ID {
		t.Fatalf("stored session = %#v", stored)
	}
}

func mkdirProjectDir(t *testing.T, base string) string {
	t.Helper()
	target := filepath.Join(t.TempDir(), base)
	if err := os.MkdirAll(target, 0o755); err != nil {
		t.Fatal(err)
	}
	return target
}

func newTestLifecycle(p paths.Paths, tmux sessionlifecycle.TmuxLauncher) *sessionlifecycle.Lifecycle {
	return sessionlifecycle.New(sessionlifecycle.Options{
		Config:   testConfig(),
		Projects: projects.NewStore(p.ProjectsFile()),
		State:    state.NewStore(p.StateFile(), p.StateLock()),
		Tmux:     tmux,
	})
}

func testConfig() config.Config {
	return config.Config{
		Agents: map[string]config.Agent{
			"claude": {Command: "sh"},
		},
	}
}

type fakeTmux struct {
	created                 []tmux.NewSessionOpts
	onNewSession            func(tmux.NewSessionOpts) error
	verifySession           bool
	hasSession              bool
	live                    map[string]bool // per-session liveness; if set takes priority over hasSession
	focusHooksInstalledWith string
	detachKeyBound          string
	killed                  []string
	killErr                 error
}

func (f *fakeTmux) NewSession(o tmux.NewSessionOpts) error {
	f.created = append(f.created, o)
	if f.onNewSession != nil {
		if err := f.onNewSession(o); err != nil {
			return err
		}
	}
	if !f.verifySession {
		f.hasSession = true
	}
	return nil
}

func (f *fakeTmux) HasSession(name string) (bool, error) {
	if f.live != nil {
		return f.live[name], nil
	}
	return f.hasSession, nil
}

func (f *fakeTmux) InstallFocusHooks(cleoBin string) error {
	f.focusHooksInstalledWith = cleoBin
	return nil
}

func (f *fakeTmux) BindDetachKey(detachKey string) error {
	f.detachKeyBound = detachKey
	return nil
}

func (f *fakeTmux) Kill(n string) error {
	f.killed = append(f.killed, n)
	if f.killErr != nil {
		return f.killErr
	}
	return nil
}
