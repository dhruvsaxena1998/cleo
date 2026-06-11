package worktree_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dhruvsaxena1998/cleo/internal/worktree"
)

// These are the one place real git runs in the test suite: integration tests
// for the adapter against throwaway `git init` repos in temp dirs. All
// lifecycle policy is tested against the fake seam in sessionlifecycle.

func gitOrSkip(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed")
	}
}

func git(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=cleo-test", "GIT_AUTHOR_EMAIL=cleo@test",
		"GIT_COMMITTER_NAME=cleo-test", "GIT_COMMITTER_EMAIL=cleo@test",
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

// initRepo creates a repo with one commit and returns its path.
func initRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	git(t, dir, "init", "-q", "-b", "main")
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("hello\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	git(t, dir, "add", ".")
	git(t, dir, "commit", "-q", "-m", "initial")
	return dir
}

func wtDir(repo, key string) string {
	return filepath.Join(repo, ".cleo", "worktrees", key)
}

func TestCreateMakesWorktreeOnNewBranchAtHead(t *testing.T) {
	gitOrSkip(t)
	repo := initRepo(t)
	c := worktree.NewClient()

	created, err := c.Create(worktree.CreateOpts{
		ProjectPath: repo,
		Dir:         wtDir(repo, "claude-fix"),
		Branch:      "cleo/wt-claude-fix",
	})
	if err != nil {
		t.Fatal(err)
	}
	if created.CWD != wtDir(repo, "claude-fix") {
		t.Fatalf("CWD = %q, want the worktree root", created.CWD)
	}
	if _, err := os.Stat(filepath.Join(created.CWD, "README.md")); err != nil {
		t.Fatalf("worktree should contain the repo files: %v", err)
	}
	branch := git(t, created.CWD, "branch", "--show-current")
	if branch != "cleo/wt-claude-fix" {
		t.Fatalf("worktree branch = %q", branch)
	}
	if git(t, created.CWD, "rev-parse", "HEAD") != git(t, repo, "rev-parse", "HEAD") {
		t.Fatal("worktree should branch off the current HEAD")
	}
}

func TestCreateWithBaseRefBranchesFromIt(t *testing.T) {
	gitOrSkip(t)
	repo := initRepo(t)
	baseCommit := git(t, repo, "rev-parse", "HEAD")
	// Advance main past the commit we'll use as base.
	if err := os.WriteFile(filepath.Join(repo, "later.txt"), []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	git(t, repo, "add", ".")
	git(t, repo, "commit", "-q", "-m", "later")
	git(t, repo, "branch", "base-branch", baseCommit)

	c := worktree.NewClient()
	created, err := c.Create(worktree.CreateOpts{
		ProjectPath: repo,
		Dir:         wtDir(repo, "claude-old"),
		Branch:      "cleo/wt-claude-old",
		Base:        "base-branch",
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := git(t, created.CWD, "rev-parse", "HEAD"); got != baseCommit {
		t.Fatalf("worktree HEAD = %s, want base commit %s", got, baseCommit)
	}
}

func TestCreateFailsOnNonGitProject(t *testing.T) {
	gitOrSkip(t)
	dir := t.TempDir()
	c := worktree.NewClient()

	_, err := c.Create(worktree.CreateOpts{
		ProjectPath: dir,
		Dir:         wtDir(dir, "claude-x"),
		Branch:      "cleo/wt-claude-x",
	})
	if err == nil || !strings.Contains(err.Error(), "git repository") {
		t.Fatalf("error = %v, want a clear not-a-git-repository error", err)
	}
}

func TestCreateFailsOnUnbornHead(t *testing.T) {
	gitOrSkip(t)
	dir := t.TempDir()
	git(t, dir, "init", "-q", "-b", "main") // no commits
	c := worktree.NewClient()

	_, err := c.Create(worktree.CreateOpts{
		ProjectPath: dir,
		Dir:         wtDir(dir, "claude-x"),
		Branch:      "cleo/wt-claude-x",
	})
	if err == nil || !strings.Contains(err.Error(), "HEAD") {
		t.Fatalf("error = %v, want a clear unborn-HEAD error", err)
	}
}

func TestCreateFailsOnUnresolvableBase(t *testing.T) {
	gitOrSkip(t)
	repo := initRepo(t)
	c := worktree.NewClient()

	_, err := c.Create(worktree.CreateOpts{
		ProjectPath: repo,
		Dir:         wtDir(repo, "claude-x"),
		Branch:      "cleo/wt-claude-x",
		Base:        "no-such-branch",
	})
	if err == nil || !strings.Contains(err.Error(), "no-such-branch") {
		t.Fatalf("error = %v, want it to name the bad base", err)
	}
}

func TestCreateForMonorepoSubdirMapsCwdIntoWorktree(t *testing.T) {
	gitOrSkip(t)
	repo := initRepo(t)
	pkg := filepath.Join(repo, "packages", "api")
	if err := os.MkdirAll(pkg, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pkg, "main.go"), []byte("package main\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	git(t, repo, "add", ".")
	git(t, repo, "commit", "-q", "-m", "add package")

	// The Project is the package subdirectory; the worktree lives under it but
	// is a full-repo worktree, and the session cwd is the matching subdir.
	c := worktree.NewClient()
	created, err := c.Create(worktree.CreateOpts{
		ProjectPath: pkg,
		Dir:         wtDir(pkg, "claude-api"),
		Branch:      "cleo/wt-claude-api",
	})
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(wtDir(pkg, "claude-api"), "packages", "api")
	if created.CWD != want {
		t.Fatalf("CWD = %q, want monorepo-mapped %q", created.CWD, want)
	}
	if _, err := os.Stat(filepath.Join(created.CWD, "main.go")); err != nil {
		t.Fatalf("mapped cwd should contain the package files: %v", err)
	}
	if _, err := os.Stat(filepath.Join(wtDir(pkg, "claude-api"), "README.md")); err != nil {
		t.Fatalf("worktree should be full-repo: %v", err)
	}
}

func TestEnsureExcludedIsIdempotentAndPreservesExistingRules(t *testing.T) {
	gitOrSkip(t)
	repo := initRepo(t)
	excludeFile := filepath.Join(repo, ".git", "info", "exclude")
	if err := os.MkdirAll(filepath.Dir(excludeFile), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(excludeFile, []byte("existing-rule\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	c := worktree.NewClient()
	if err := c.EnsureExcluded(repo); err != nil {
		t.Fatal(err)
	}
	if err := c.EnsureExcluded(repo); err != nil {
		t.Fatal(err)
	}

	b, err := os.ReadFile(excludeFile)
	if err != nil {
		t.Fatal(err)
	}
	content := string(b)
	if !strings.Contains(content, "existing-rule") {
		t.Fatalf("existing rules clobbered: %q", content)
	}
	if got := strings.Count(content, ".cleo/"); got != 1 {
		t.Fatalf(".cleo/ appears %d times, want exactly once:\n%s", got, content)
	}
	// The exclusion must actually take: .cleo/ content invisible to status.
	if err := os.MkdirAll(filepath.Join(repo, ".cleo", "worktrees"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, ".cleo", "worktrees", "x.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if status := git(t, repo, "status", "--porcelain"); strings.Contains(status, ".cleo") {
		t.Fatalf(".cleo/ shows in git status: %q", status)
	}
}

func TestIsDirty(t *testing.T) {
	gitOrSkip(t)
	repo := initRepo(t)
	c := worktree.NewClient()
	created, err := c.Create(worktree.CreateOpts{
		ProjectPath: repo,
		Dir:         wtDir(repo, "claude-d"),
		Branch:      "cleo/wt-claude-d",
	})
	if err != nil {
		t.Fatal(err)
	}

	if dirty, err := c.IsDirty(created.CWD); err != nil || dirty {
		t.Fatalf("fresh worktree dirty=%v err=%v, want clean", dirty, err)
	}

	if err := os.WriteFile(filepath.Join(created.CWD, "untracked.txt"), []byte("wip"), 0o644); err != nil {
		t.Fatal(err)
	}
	if dirty, err := c.IsDirty(created.CWD); err != nil || !dirty {
		t.Fatalf("untracked file dirty=%v err=%v, want dirty", dirty, err)
	}

	if dirty, err := c.IsDirty(filepath.Join(repo, "no-such-dir")); err != nil || dirty {
		t.Fatalf("missing dir dirty=%v err=%v, want clean (manual deletion is tolerated)", dirty, err)
	}
}

func TestRemoveDeletesWorktreeButNeverTheBranch(t *testing.T) {
	gitOrSkip(t)
	repo := initRepo(t)
	c := worktree.NewClient()
	dir := wtDir(repo, "claude-rm")
	if _, err := c.Create(worktree.CreateOpts{
		ProjectPath: repo, Dir: dir, Branch: "cleo/wt-claude-rm",
	}); err != nil {
		t.Fatal(err)
	}

	if err := c.Remove(worktree.RemoveOpts{ProjectPath: repo, Dir: dir}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Fatalf("worktree dir should be gone, stat err=%v", err)
	}
	branches := git(t, repo, "branch", "--list", "cleo/wt-claude-rm")
	if !strings.Contains(branches, "cleo/wt-claude-rm") {
		t.Fatal("branch must survive worktree removal")
	}
}

func TestRemoveRefusesDirtyWorktreeUnlessForced(t *testing.T) {
	gitOrSkip(t)
	repo := initRepo(t)
	c := worktree.NewClient()
	dir := wtDir(repo, "claude-dirty")
	if _, err := c.Create(worktree.CreateOpts{
		ProjectPath: repo, Dir: dir, Branch: "cleo/wt-claude-dirty",
	}); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "wip.txt"), []byte("wip"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := c.Remove(worktree.RemoveOpts{ProjectPath: repo, Dir: dir}); err == nil {
		t.Fatal("unforced remove of a dirty worktree should fail (git's safety model)")
	}
	if err := c.Remove(worktree.RemoveOpts{ProjectPath: repo, Dir: dir, Force: true}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Fatal("forced remove should delete the dirty worktree")
	}
}

func TestRemoveToleratesManuallyDeletedWorktree(t *testing.T) {
	gitOrSkip(t)
	repo := initRepo(t)
	c := worktree.NewClient()
	dir := wtDir(repo, "claude-gone")
	if _, err := c.Create(worktree.CreateOpts{
		ProjectPath: repo, Dir: dir, Branch: "cleo/wt-claude-gone",
	}); err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(dir); err != nil {
		t.Fatal(err)
	}

	if err := c.Remove(worktree.RemoveOpts{ProjectPath: repo, Dir: dir}); err != nil {
		t.Fatalf("cleanup after manual deletion should succeed, got %v", err)
	}
	// Metadata is pruned: git no longer lists the worktree.
	if list := git(t, repo, "worktree", "list"); strings.Contains(list, "claude-gone") {
		t.Fatalf("stale worktree metadata left behind: %q", list)
	}
}

func TestRemoveWithDeleteBranchRollsBackTheBranch(t *testing.T) {
	gitOrSkip(t)
	repo := initRepo(t)
	c := worktree.NewClient()
	dir := wtDir(repo, "claude-rb")
	if _, err := c.Create(worktree.CreateOpts{
		ProjectPath: repo, Dir: dir, Branch: "cleo/wt-claude-rb",
	}); err != nil {
		t.Fatal(err)
	}

	if err := c.Remove(worktree.RemoveOpts{
		ProjectPath: repo, Dir: dir, Force: true, DeleteBranch: "cleo/wt-claude-rb",
	}); err != nil {
		t.Fatal(err)
	}
	if branches := git(t, repo, "branch", "--list", "cleo/wt-claude-rb"); branches != "" {
		t.Fatalf("rollback left branch behind: %q", branches)
	}
	// A retry under the same name must succeed.
	if _, err := c.Create(worktree.CreateOpts{
		ProjectPath: repo, Dir: dir, Branch: "cleo/wt-claude-rb",
	}); err != nil {
		t.Fatalf("retry after rollback failed: %v", err)
	}
}
