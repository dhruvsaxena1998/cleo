package cli

import (
	"encoding/json"
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/dhruvsaxena1998/cleo/internal/config"
	"github.com/dhruvsaxena1998/cleo/internal/projects"
	"github.com/dhruvsaxena1998/cleo/internal/tmux"
	"github.com/dhruvsaxena1998/cleo/internal/worktree"
)

func testRootedCtx(t *testing.T, root string) func() *Ctx {
	t.Helper()
	return func() *Ctx {
		c, err := NewCtxWithRoot(root)
		if err != nil {
			t.Fatal(err)
		}
		return c
	}
}

func mkdir(p string) error { return os.MkdirAll(p, 0o755) }

func usePortableAgentCommand(c *Ctx, agentName string) {
	agent := c.Config.Agents[agentName]
	agent.Command = "sh"
	if c.Config.Agents == nil {
		c.Config.Agents = map[string]config.Agent{}
	}
	c.Config.Agents[agentName] = agent
}

// setProjectDefaultWorktree flips DefaultWorktree the supported way: editing
// projects.json on disk (there is deliberately no setter command in v1).
func setProjectDefaultWorktree(t *testing.T, c *Ctx, projectID string) {
	t.Helper()
	all, err := c.Projects.List()
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
	if err := os.WriteFile(c.Paths.ProjectsFile(), b, 0o644); err != nil {
		t.Fatal(err)
	}
}

// fakeWorktree satisfies the lifecycle's Worktree seam for CLI tests.
type fakeWorktree struct {
	created  []worktree.CreateOpts
	removed  []worktree.RemoveOpts
	excluded []string
	dirty    map[string]bool // by Dir
}

func (f *fakeWorktree) Create(o worktree.CreateOpts) (worktree.Created, error) {
	f.created = append(f.created, o)
	return worktree.Created{CWD: o.Dir}, nil
}

func (f *fakeWorktree) Remove(o worktree.RemoveOpts) error {
	f.removed = append(f.removed, o)
	return nil
}

func (f *fakeWorktree) IsDirty(dir string) (bool, error) { return f.dirty[dir], nil }

func (f *fakeWorktree) EnsureExcluded(projectPath string) error {
	f.excluded = append(f.excluded, projectPath)
	return nil
}

type fakeTmux struct {
	created  []tmux.NewSessionOpts
	exists   map[string]bool
	attached []string
}

func (f *fakeTmux) NewSession(o tmux.NewSessionOpts) error {
	f.created = append(f.created, o)
	if f.exists == nil {
		f.exists = map[string]bool{}
	}
	f.exists[o.Name] = true
	return nil
}
func (f *fakeTmux) HasSession(n string) (bool, error) { return f.exists[n], nil }
func (f *fakeTmux) LsPrefix(p string) ([]string, error) {
	var out []string
	for k := range f.exists {
		if strings.HasPrefix(k, p) {
			out = append(out, k)
		}
	}
	return out, nil
}
func (f *fakeTmux) Kill(n string) error                     { delete(f.exists, n); return nil }
func (f *fakeTmux) BindDetachKey(string) error              { return nil }
func (f *fakeTmux) InstallFocusHooks(string) error          { return nil }
func (f *fakeTmux) CapturePane(string, int) (string, error) { return "", nil }
func (f *fakeTmux) SendKeys(string, string) error           { return nil }
func (f *fakeTmux) RenameSession(from, to string) error     { return nil }
func (f *fakeTmux) SessionPIDs(name string) ([]int, error)  { return nil, nil }
func (f *fakeTmux) AttachCmd(sessionID string) *exec.Cmd {
	f.attached = append(f.attached, sessionID)
	return exec.Command("true") // harmless no-op; records the attach request
}
