package cli

import (
	"github.com/dhruvsaxena1998/cleo/internal/config"
	"github.com/dhruvsaxena1998/cleo/internal/events"
	"github.com/dhruvsaxena1998/cleo/internal/paths"
	"github.com/dhruvsaxena1998/cleo/internal/projects"
	"github.com/dhruvsaxena1998/cleo/internal/sound"
	"github.com/dhruvsaxena1998/cleo/internal/state"
	"github.com/dhruvsaxena1998/cleo/internal/tmux"
)

type Ctx struct {
	Paths    paths.Paths
	Config   config.Config
	Projects *projects.Store
	State    *state.Store
	Tmux     *tmux.Client
	Player   *sound.Player
	Events   func(sid string) *events.Log
}

func NewCtx() (*Ctx, error) { return NewCtxWithRoot(paths.New().ConfigDir()) }

func NewCtxWithRoot(root string) (*Ctx, error) {
	p := paths.NewWithRoot(root)
	cfg, err := config.Load(p.ConfigFile())
	if err != nil {
		return nil, err
	}
	return &Ctx{
		Paths:    p,
		Config:   cfg,
		Projects: projects.NewStore(p.ProjectsFile()),
		State:    state.NewStore(p.StateFile(), p.StateLock()),
		Tmux:     tmux.NewClient(""),
		Player:   sound.NewPlayer(cfg.Sound.Volume),
		Events:   func(sid string) *events.Log { return events.NewLog(p.EventsLog(sid)) },
	}, nil
}
