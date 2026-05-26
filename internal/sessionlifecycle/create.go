package sessionlifecycle

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/dhruvsaxena1998/cleo/internal/config"
	"github.com/dhruvsaxena1998/cleo/internal/ids"
	"github.com/dhruvsaxena1998/cleo/internal/projects"
	"github.com/dhruvsaxena1998/cleo/internal/state"
	"github.com/dhruvsaxena1998/cleo/internal/tmux"
)

var (
	ErrProjectRegistrationNeeded = errors.New("project registration needed")
	ErrUnknownAgent              = errors.New("unknown agent")
)

type TmuxLauncher interface {
	NewSession(tmux.NewSessionOpts) error
}

type Options struct {
	Config       config.Config
	Projects     *projects.Store
	State        *state.Store
	Tmux         TmuxLauncher
	CleoBin      string
	GenerateName func(existing map[string]bool) string
}

type Lifecycle struct {
	cfg          config.Config
	projects     *projects.Store
	state        *state.Store
	tmux         TmuxLauncher
	cleoBin      string
	generateName func(existing map[string]bool) string
}

func New(opts Options) *Lifecycle {
	generateName := opts.GenerateName
	if generateName == nil {
		generateName = ids.RandomName
	}
	return &Lifecycle{
		cfg:          opts.Config,
		projects:     opts.Projects,
		state:        opts.State,
		tmux:         opts.Tmux,
		cleoBin:      opts.CleoBin,
		generateName: generateName,
	}
}

type CreateInput struct {
	Agent               string
	Name                string
	Path                string
	ProjectID           string
	AutoRegisterProject bool
}

type CreateResult struct {
	Session           state.Session
	Project           projects.Project
	ProjectRegistered bool
}

func (l *Lifecycle) Create(input CreateInput) (CreateResult, error) {
	agent, ok := l.cfg.Agents[input.Agent]
	if !ok {
		return CreateResult{}, ErrUnknownAgent
	}

	proj, registered, err := l.resolveProject(input)
	if err != nil {
		return CreateResult{}, err
	}

	existing, err := l.existingSessionNames(proj.ID, input.Agent)
	if err != nil {
		return CreateResult{}, err
	}
	name := ids.DedupeSlug(l.generateName(existing), existing)
	if input.Name != "" {
		name = ids.DedupeSlug(ids.Slugify(input.Name), existing)
	}
	sid := ids.MakeSessionID(proj.ID, input.Agent, name)
	sess := state.Session{
		ID:        sid,
		ProjectID: proj.ID,
		Agent:     input.Agent,
		Name:      name,
		State:     state.Spawning,
		StartedAt: time.Now().UTC(),
	}
	if err := l.state.Put(sess); err != nil {
		return CreateResult{}, err
	}
	if err := l.tmux.NewSession(tmux.NewSessionOpts{
		Name: sid,
		Cwd:  proj.Path,
		Cmd:  agent.Command,
		Env:  map[string]string{"CLEO_SESSION_ID": sid},
	}); err != nil {
		_ = l.state.Delete(sid)
		return CreateResult{}, err
	}
	l.installFocusHooks()
	l.bindDetachKey()
	return CreateResult{Session: sess, Project: proj, ProjectRegistered: registered}, nil
}

func (l *Lifecycle) bindDetachKey() {
	if l.cfg.Tmux.DetachKey == "" {
		return
	}
	binder, ok := l.tmux.(interface{ BindDetachKey(string) error })
	if !ok {
		return
	}
	_ = binder.BindDetachKey(l.cfg.Tmux.DetachKey)
}

func (l *Lifecycle) installFocusHooks() {
	installer, ok := l.tmux.(interface{ InstallFocusHooks(string) error })
	if !ok {
		return
	}
	cleoBin := l.cleoBin
	if cleoBin == "" {
		var err error
		cleoBin, err = os.Executable()
		if err != nil {
			return
		}
		cleoBin, _ = filepath.Abs(cleoBin)
	}
	_ = installer.InstallFocusHooks(cleoBin)
}

func (l *Lifecycle) resolveProject(input CreateInput) (projects.Project, bool, error) {
	if input.ProjectID != "" {
		proj, err := l.projects.Get(input.ProjectID)
		return proj, false, err
	}

	path := input.Path
	if path != "" {
		path, _ = filepath.Abs(path)
	}
	proj, err := l.projects.ResolveFromCwd(path)
	if errors.Is(err, projects.ErrNotFound) {
		if !input.AutoRegisterProject {
			return projects.Project{}, false, ErrProjectRegistrationNeeded
		}
		proj, err = l.projects.Add(path)
		return proj, err == nil, err
	}
	return proj, false, err
}

func (l *Lifecycle) existingSessionNames(projectID, agent string) (map[string]bool, error) {
	out := map[string]bool{}
	sessions, err := l.state.List()
	if err != nil {
		return nil, err
	}
	prefix := fmt.Sprintf("cleo-%s-%s-", projectID, agent)
	for _, sess := range sessions {
		if strings.HasPrefix(sess.ID, prefix) {
			out[sess.Name] = true
		}
	}
	return out, nil
}
