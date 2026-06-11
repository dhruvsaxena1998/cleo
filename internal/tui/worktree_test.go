package tui

import (
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/dhruvsaxena1998/cleo/internal/projects"
	"github.com/dhruvsaxena1998/cleo/internal/state"
	"github.com/dhruvsaxena1998/cleo/internal/worktree"
)

// fakeWorktree satisfies the lifecycle's Worktree seam for TUI tests.
type fakeWorktree struct {
	created []worktree.CreateOpts
}

func (f *fakeWorktree) Create(o worktree.CreateOpts) (worktree.Created, error) {
	f.created = append(f.created, o)
	return worktree.Created{CWD: o.Dir}, nil
}
func (f *fakeWorktree) Remove(worktree.RemoveOpts) error { return nil }
func (f *fakeWorktree) IsDirty(string) (bool, error)     { return false, nil }
func (f *fakeWorktree) EnsureExcluded(string) error      { return nil }

// ── Spawn popup toggle ───────────────────────────────────────────────────────

func TestSpawnPopupWorktreeTogglePresetFromProjectDefault(t *testing.T) {
	projs := []projects.Project{
		{ID: "isolated", Path: "/x/isolated", DefaultWorktree: true},
		{ID: "plain", Path: "/x/plain"},
	}
	on := NewSpawnPopup("isolated", projs, "/tmp", []string{"claude"}, "", Resolve("catppuccin"))
	if !on.worktree {
		t.Fatal("toggle should be pre-set from the project's DefaultWorktree")
	}
	off := NewSpawnPopup("plain", projs, "/tmp", []string{"claude"}, "", Resolve("catppuccin"))
	if off.worktree {
		t.Fatal("toggle should default off without a project default")
	}
}

func TestSpawnPopupWorktreeToggleFlipsWhenFocused(t *testing.T) {
	popup := newTestSpawnPopup(t)
	// Tab to the worktree zone: path → label → agents → worktree.
	var model tea.Model = popup
	for i := 0; i < 3; i++ {
		model, _ = model.(SpawnPopup).Update(tea.KeyMsg{Type: tea.KeyTab})
	}
	popup = model.(SpawnPopup)
	if popup.focusIndex != 3 {
		t.Fatalf("focusIndex = %d, want 3 (worktree)", popup.focusIndex)
	}

	model, _ = popup.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("l")})
	popup = model.(SpawnPopup)
	if !popup.worktree {
		t.Fatal("toggle should flip on")
	}
	model, _ = popup.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("h")})
	popup = model.(SpawnPopup)
	if popup.worktree {
		t.Fatal("toggle should flip back off")
	}
}

func TestSpawnPopupSubmitCarriesWorktreeChoice(t *testing.T) {
	dir := t.TempDir()
	popup := newTestSpawnPopup(t)
	popup.pathInput.SetValue(dir)
	popup.worktree = true

	model, cmd := popup.Update(tea.KeyMsg{Type: tea.KeyEnter})
	_ = model
	if cmd == nil {
		t.Fatal("expected submit")
	}
	msg := cmd()
	submitted, ok := msg.(SpawnSubmitted)
	if !ok {
		t.Fatalf("msg = %#v, want SpawnSubmitted", msg)
	}
	if !submitted.Worktree {
		t.Fatal("SpawnSubmitted should carry the worktree choice")
	}
}

// ── performSpawn wiring ──────────────────────────────────────────────────────

func TestPerformSpawnHonorsWorktreeToggle(t *testing.T) {
	c := newTestCtx(t)
	target := filepath.Join(t.TempDir(), "myapp")
	if err := mkdirAll(target); err != nil {
		t.Fatal(err)
	}
	proj, err := c.Projects.Add(target)
	if err != nil {
		t.Fatal(err)
	}
	wt := &fakeWorktree{}
	c.Worktree = wt

	m := New(c)
	model, _ := m.Update(SpawnSubmitted{
		ProjectID: proj.ID,
		Path:      target,
		Agent:     "claude",
		Name:      "isolated",
		Worktree:  true,
	})
	_ = model

	if len(wt.created) != 1 {
		t.Fatalf("worktree creations = %#v, want one", wt.created)
	}
	sess, err := c.State.Get("cleo-myapp-claude-isolated")
	if err != nil {
		t.Fatal(err)
	}
	if !sess.HasWorktree() {
		t.Fatalf("session record missing worktree fields: %#v", sess)
	}
}

// ── Badges ───────────────────────────────────────────────────────────────────

func worktreeTestSession(target string) state.Session {
	return state.Session{
		ID: "cleo-myapp-claude-iso", ProjectID: "myapp", Agent: "claude", Name: "iso",
		State:          state.Running,
		WorktreePath:   filepath.Join(target, ".cleo", "worktrees", "claude-iso"),
		WorktreeBranch: "cleo/wt-claude-iso",
	}
}

func TestSidebarShowsWorktreeBadge(t *testing.T) {
	c := newTestCtx(t)
	target := filepath.Join(t.TempDir(), "myapp")
	_ = mkdirAll(target)
	_, _ = c.Projects.Add(target)
	_ = c.State.Put(worktreeTestSession(target))

	m := New(c)
	m.width, m.height = 120, 40
	projs, _ := c.Projects.List()
	sessions, _ := c.State.List()
	m.projects = projs
	m.sessions = sessions
	m.expanded = map[string]bool{"myapp": true}

	tree := m.renderTreeContent(40)
	badge := m.theme.Icons.Branch
	if badge == "" {
		badge = "wt"
	}
	if !strings.Contains(tree, badge) {
		t.Fatalf("sidebar should badge worktree sessions, got:\n%s", tree)
	}
}

func TestSessionDetailShowsWorktreeBranch(t *testing.T) {
	c := newTestCtx(t)
	target := filepath.Join(t.TempDir(), "myapp")
	_ = mkdirAll(target)
	_, _ = c.Projects.Add(target)
	sess := worktreeTestSession(target)
	_ = c.State.Put(sess)

	m := New(c)
	detail := m.renderSessionDetail(sess, 100)
	if !strings.Contains(detail, "cleo/wt-claude-iso") {
		t.Fatalf("detail pane should show the worktree branch, got:\n%s", detail)
	}
}
