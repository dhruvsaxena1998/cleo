package sessionlifecycle_test

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dhruvsaxena1998/cleo/internal/paths"
	"github.com/dhruvsaxena1998/cleo/internal/projects"
	"github.com/dhruvsaxena1998/cleo/internal/sessionlifecycle"
	"github.com/dhruvsaxena1998/cleo/internal/state"
	"github.com/dhruvsaxena1998/cleo/internal/tmux"
	"github.com/dhruvsaxena1998/cleo/internal/worktree"
)

// fakeWorktree satisfies the lifecycle's Worktree seam in tests.
type fakeWorktree struct {
	created    []worktree.CreateOpts
	removed    []worktree.RemoveOpts
	excluded   []string
	calls      []string // operation order: "exclude", "create", "remove", "dirty"
	createErr  error
	excludeErr error
	removeErr  error
	dirty      map[string]bool // by Dir
	dirtyErr   error
	cwd        func(worktree.CreateOpts) string // default: opts.Dir
}

func (f *fakeWorktree) Create(o worktree.CreateOpts) (worktree.Created, error) {
	f.calls = append(f.calls, "create")
	f.created = append(f.created, o)
	if f.createErr != nil {
		return worktree.Created{}, f.createErr
	}
	cwd := o.Dir
	if f.cwd != nil {
		cwd = f.cwd(o)
	}
	return worktree.Created{CWD: cwd}, nil
}

func (f *fakeWorktree) Remove(o worktree.RemoveOpts) error {
	f.calls = append(f.calls, "remove")
	f.removed = append(f.removed, o)
	return f.removeErr
}

func (f *fakeWorktree) IsDirty(dir string) (bool, error) {
	f.calls = append(f.calls, "dirty")
	return f.dirty[dir], f.dirtyErr
}

func (f *fakeWorktree) EnsureExcluded(projectPath string) error {
	f.calls = append(f.calls, "exclude")
	f.excluded = append(f.excluded, projectPath)
	return f.excludeErr
}

func boolPtr(b bool) *bool { return &b }

// worktreeHarness wires a lifecycle with a registered project, a fake tmux,
// and a fake Worktree seam.
type worktreeHarness struct {
	Paths       paths.Paths
	ProjectID   string
	ProjectPath string
	State       *state.Store
	Tmux        *fakeTmux
	Worktree    *fakeWorktree
	Lifecycle   *sessionlifecycle.Lifecycle
}

func newWorktreeHarness(t *testing.T) *worktreeHarness {
	t.Helper()
	p := paths.NewWithRoot(t.TempDir())
	projectPath := mkdirProjectDir(t, "myapp")
	projectStore := projects.NewStore(p.ProjectsFile())
	registered, err := projectStore.Add(projectPath)
	if err != nil {
		t.Fatal(err)
	}
	fake := &fakeTmux{}
	wt := &fakeWorktree{}
	l := sessionlifecycle.New(sessionlifecycle.Options{
		Config:   testConfig(),
		Projects: projectStore,
		State:    state.NewStore(p.StateFile(), p.StateLock()),
		Tmux:     fake,
		Worktree: wt,
	})
	return &worktreeHarness{
		Paths:       p,
		ProjectID:   registered.ID,
		ProjectPath: projectPath,
		State:       state.NewStore(p.StateFile(), p.StateLock()),
		Tmux:        fake,
		Worktree:    wt,
		Lifecycle:   l,
	}
}

func TestCreateWorktreeOptInPrecedence(t *testing.T) {
	cases := []struct {
		name           string
		projectDefault bool
		flag           *bool
		want           bool
	}{
		{"off by default", false, nil, false},
		{"explicit flag enables", false, boolPtr(true), true},
		{"project default enables", true, nil, true},
		{"no-worktree overrides project default", true, boolPtr(false), false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := paths.NewWithRoot(t.TempDir())
			projectPath := mkdirProjectDir(t, "myapp")
			projectStore := projects.NewStore(p.ProjectsFile())
			registered, err := projectStore.Add(projectPath)
			if err != nil {
				t.Fatal(err)
			}
			if tc.projectDefault {
				setProjectDefaultWorktree(t, p, registered.ID)
			}
			fake := &fakeTmux{}
			wt := &fakeWorktree{}
			l := sessionlifecycle.New(sessionlifecycle.Options{
				Config:   testConfig(),
				Projects: projectStore,
				State:    state.NewStore(p.StateFile(), p.StateLock()),
				Tmux:     fake,
				Worktree: wt,
			})

			result, err := l.Create(sessionlifecycle.CreateInput{
				Agent:     "claude",
				Name:      "precedence",
				ProjectID: registered.ID,
				Worktree:  tc.flag,
			})
			if err != nil {
				t.Fatal(err)
			}

			if got := len(wt.created) == 1; got != tc.want {
				t.Fatalf("worktree created = %v, want %v (creations: %#v)", got, tc.want, wt.created)
			}
			if result.Session.HasWorktree() != tc.want {
				t.Fatalf("session HasWorktree = %v, want %v", result.Session.HasWorktree(), tc.want)
			}
			if !tc.want {
				if len(fake.created) != 1 || fake.created[0].Cwd != projectPath {
					t.Fatalf("main-tree spawn cwd = %#v, want project path", fake.created)
				}
			}
		})
	}
}

func TestCreateFailsBeforeTmuxWhenWorktreeCreationFails(t *testing.T) {
	h := newWorktreeHarness(t)
	h.Worktree.createErr = errors.New("not a git repository")

	_, err := h.Lifecycle.Create(sessionlifecycle.CreateInput{
		Agent:     "claude",
		Name:      "no-repo",
		ProjectID: h.ProjectID,
		Worktree:  boolPtr(true),
	})
	if !errors.Is(err, sessionlifecycle.ErrWorktreeFailed) {
		t.Fatalf("Create error = %v, want ErrWorktreeFailed", err)
	}
	if !strings.Contains(err.Error(), "not a git repository") {
		t.Fatalf("Create error = %q, want the adapter's reason included", err.Error())
	}
	if len(h.Tmux.created) != 0 {
		t.Fatalf("tmux session created despite worktree failure: %#v", h.Tmux.created)
	}
	if sessions, err := h.State.List(); err != nil {
		t.Fatal(err)
	} else if len(sessions) != 0 {
		t.Fatalf("failed worktree spawn wrote session records: %#v", sessions)
	}
}

func TestCreateRollsBackWorktreeWhenTmuxLaunchFails(t *testing.T) {
	h := newWorktreeHarness(t)
	h.Tmux.onNewSession = func(tmux.NewSessionOpts) error {
		return errors.New("tmux refused session")
	}

	_, err := h.Lifecycle.Create(sessionlifecycle.CreateInput{
		Agent:     "claude",
		Name:      "will-fail",
		ProjectID: h.ProjectID,
		Worktree:  boolPtr(true),
	})
	if err == nil {
		t.Fatal("Create succeeded despite tmux failure")
	}

	assertWorktreeRolledBack(t, h, "claude-will-fail")
}

func TestCreateRollsBackWorktreeWhenSessionExitsImmediately(t *testing.T) {
	h := newWorktreeHarness(t)
	h.Tmux.verifySession = true
	h.Tmux.hasSession = false

	_, err := h.Lifecycle.Create(sessionlifecycle.CreateInput{
		Agent:     "claude",
		Name:      "exits-immediately",
		ProjectID: h.ProjectID,
		Worktree:  boolPtr(true),
	})
	if !errors.Is(err, sessionlifecycle.ErrLaunchFailed) {
		t.Fatalf("Create error = %v, want ErrLaunchFailed", err)
	}

	assertWorktreeRolledBack(t, h, "claude-exits-immediately")
}

// assertWorktreeRolledBack checks the full inverse of a worktree spawn: the
// record is gone and the just-created worktree and branch are removed.
func assertWorktreeRolledBack(t *testing.T, h *worktreeHarness, key string) {
	t.Helper()
	if sessions, err := h.State.List(); err != nil {
		t.Fatal(err)
	} else if len(sessions) != 0 {
		t.Fatalf("failed spawn left session records: %#v", sessions)
	}
	if len(h.Worktree.removed) != 1 {
		t.Fatalf("worktree removals = %#v, want exactly one rollback", h.Worktree.removed)
	}
	got := h.Worktree.removed[0]
	if got.Dir != filepath.Join(h.ProjectPath, ".cleo", "worktrees", key) {
		t.Fatalf("rollback dir = %q", got.Dir)
	}
	if got.ProjectPath != h.ProjectPath {
		t.Fatalf("rollback project path = %q", got.ProjectPath)
	}
	if !got.Force {
		t.Fatal("rollback should force-remove the just-created worktree")
	}
	if got.DeleteBranch != "cleo/wt-"+key {
		t.Fatalf("rollback DeleteBranch = %q, want the just-created branch (a retry with the same name must not collide)", got.DeleteBranch)
	}
}

func TestCreateWithWorktreeMaintainsRepoExcludeBeforeCreatingWorktree(t *testing.T) {
	h := newWorktreeHarness(t)

	_, err := h.Lifecycle.Create(sessionlifecycle.CreateInput{
		Agent:     "claude",
		Name:      "hygiene",
		ProjectID: h.ProjectID,
		Worktree:  boolPtr(true),
	})
	if err != nil {
		t.Fatal(err)
	}

	if len(h.Worktree.excluded) != 1 || h.Worktree.excluded[0] != h.ProjectPath {
		t.Fatalf("EnsureExcluded calls = %#v, want one for the project path", h.Worktree.excluded)
	}
	if len(h.Worktree.calls) < 2 || h.Worktree.calls[0] != "exclude" || h.Worktree.calls[1] != "create" {
		t.Fatalf("operation order = %v, want exclude before create so .cleo/ never appears untracked", h.Worktree.calls)
	}
}

func TestCreateWithWorktreeSurvivesExcludeFailureWithWarning(t *testing.T) {
	h := newWorktreeHarness(t)
	h.Worktree.excludeErr = errors.New("info/exclude not writable")

	result, err := h.Lifecycle.Create(sessionlifecycle.CreateInput{
		Agent:     "claude",
		Name:      "hygiene-warn",
		ProjectID: h.ProjectID,
		Worktree:  boolPtr(true),
	})
	if err != nil {
		t.Fatalf("exclude maintenance is hygiene, not a spawn precondition: %v", err)
	}
	if result.Warning == nil || !strings.Contains(result.Warning.Error(), "info/exclude not writable") {
		t.Fatalf("Warning = %v, want the exclude failure surfaced", result.Warning)
	}
	if len(h.Worktree.created) != 1 {
		t.Fatalf("worktree should still be created, got %#v", h.Worktree.created)
	}
}

func TestCreateMainTreeSessionNeverTouchesExclude(t *testing.T) {
	h := newWorktreeHarness(t)

	if _, err := h.Lifecycle.Create(sessionlifecycle.CreateInput{
		Agent:     "claude",
		Name:      "plain",
		ProjectID: h.ProjectID,
	}); err != nil {
		t.Fatal(err)
	}
	if len(h.Worktree.calls) != 0 {
		t.Fatalf("main-tree spawn called the worktree seam: %v", h.Worktree.calls)
	}
}

func TestCreateWithWorktreePassesBaseRefThrough(t *testing.T) {
	h := newWorktreeHarness(t)

	_, err := h.Lifecycle.Create(sessionlifecycle.CreateInput{
		Agent:     "claude",
		Name:      "from-main",
		ProjectID: h.ProjectID,
		Worktree:  boolPtr(true),
		Base:      "main",
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(h.Worktree.created) != 1 || h.Worktree.created[0].Base != "main" {
		t.Fatalf("worktree CreateOpts = %#v, want Base=main", h.Worktree.created)
	}
}

func TestCreateRejectsBaseWithoutWorktree(t *testing.T) {
	h := newWorktreeHarness(t)

	_, err := h.Lifecycle.Create(sessionlifecycle.CreateInput{
		Agent:     "claude",
		Name:      "base-no-wt",
		ProjectID: h.ProjectID,
		Base:      "main",
	})
	if err == nil || !strings.Contains(err.Error(), "--base") {
		t.Fatalf("Create error = %v, want a clear --base-requires-worktree error", err)
	}
	if len(h.Tmux.created) != 0 || len(h.Worktree.calls) != 0 {
		t.Fatal("rejected spawn should not touch tmux or the worktree seam")
	}
	if sessions, _ := h.State.List(); len(sessions) != 0 {
		t.Fatalf("rejected spawn wrote session records: %#v", sessions)
	}
}

// setProjectDefaultWorktree flips DefaultWorktree the supported way: editing
// projects.json on disk (there is deliberately no setter command in v1).
func setProjectDefaultWorktree(t *testing.T, p paths.Paths, projectID string) {
	t.Helper()
	store := projects.NewStore(p.ProjectsFile())
	all, err := store.List()
	if err != nil {
		t.Fatal(err)
	}
	for i := range all {
		if all[i].ID == projectID {
			all[i].DefaultWorktree = true
		}
	}
	b, err := json.Marshal(struct {
		Projects []projects.Project `json:"projects"`
	}{all})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p.ProjectsFile(), b, 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestCreateWithWorktreeSpawnsSessionInsideWorktree(t *testing.T) {
	h := newWorktreeHarness(t)
	wantDir := filepath.Join(h.ProjectPath, ".cleo", "worktrees", "claude-fix-auth-bug")
	h.Worktree.cwd = func(o worktree.CreateOpts) string {
		return filepath.Join(o.Dir, "mapped-subdir")
	}

	result, err := h.Lifecycle.Create(sessionlifecycle.CreateInput{
		Agent:     "claude",
		Name:      "Fix Auth Bug",
		ProjectID: h.ProjectID,
		Worktree:  boolPtr(true),
	})
	if err != nil {
		t.Fatal(err)
	}

	if len(h.Worktree.created) != 1 {
		t.Fatalf("worktree creations = %#v, want exactly one", h.Worktree.created)
	}
	got := h.Worktree.created[0]
	want := worktree.CreateOpts{
		ProjectPath: h.ProjectPath,
		Dir:         wantDir,
		Branch:      "cleo/wt-claude-fix-auth-bug",
	}
	if got != want {
		t.Fatalf("worktree CreateOpts = %#v, want %#v", got, want)
	}

	if result.Session.WorktreePath != wantDir || result.Session.WorktreeBranch != want.Branch {
		t.Fatalf("session worktree fields = %q/%q, want %q/%q",
			result.Session.WorktreePath, result.Session.WorktreeBranch, wantDir, want.Branch)
	}

	if len(h.Tmux.created) != 1 || h.Tmux.created[0].Cwd != filepath.Join(wantDir, "mapped-subdir") {
		t.Fatalf("tmux created = %#v, want one launch with cwd inside the worktree", h.Tmux.created)
	}

	stored, err := h.State.Get(result.Session.ID)
	if err != nil {
		t.Fatal(err)
	}
	if stored.WorktreePath != wantDir || stored.WorktreeBranch != want.Branch {
		t.Fatalf("stored worktree fields = %q/%q", stored.WorktreePath, stored.WorktreeBranch)
	}
}
