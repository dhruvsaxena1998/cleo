package cli

import (
	"fmt"
	"os"

	"github.com/dhruvsaxena1998/cleo/internal/config"
	"github.com/dhruvsaxena1998/cleo/internal/events"
	"github.com/dhruvsaxena1998/cleo/internal/focus"
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
	Focus    *focus.Store
	Tmux     TmuxClient
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
	if err := sound.ExtractDefaults(p.SoundsDir()); err != nil {
		return nil, err
	}
	for _, warning := range cfg.Warnings {
		fmt.Fprintf(os.Stderr, "cleo config warning: %s\n", warning)
	}
	return &Ctx{
		Paths:    p,
		Config:   cfg,
		Projects: projects.NewStore(p.ProjectsFile()),
		State:    state.NewStore(p.StateFile(), p.StateLock()),
		Focus:    focus.NewStore(p.FocusFile()),
		Tmux:     tmux.NewClient(""),
		Player:   sound.NewPlayer(cfg.Sound.Volume),
		Events:   func(sid string) *events.Log { return events.NewLog(p.EventsLog(sid)) },
	}, nil
}
