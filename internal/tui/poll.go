// internal/tui/poll.go
package tui

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/dhruvsaxena1998/cleo/internal/cli"
	"github.com/dhruvsaxena1998/cleo/internal/projects"
	"github.com/dhruvsaxena1998/cleo/internal/reconcile"
	"github.com/dhruvsaxena1998/cleo/internal/state"
)

type stateLoadedMsg struct {
	projects []projects.Project
	sessions []state.Session
}

type tickStateMsg struct{}

type paneCapturedMsg struct {
	sid     string
	content string
}

func loadStateCmd(c *cli.Ctx) tea.Cmd {
	return func() tea.Msg {
		_ = reconcile.RunOpts(c.State, c.Tmux, reconcile.Options{
			IdleTimeout:     c.Config.Timeouts.IdleToCompletedTimeout,
			SpawningTimeout: c.Config.Timeouts.SpawningTimeout,
		})
		ps, _ := c.Projects.List()
		ss, _ := c.State.List()
		return stateLoadedMsg{projects: ps, sessions: ss}
	}
}

func tickStateCmd() tea.Cmd {
	return tea.Tick(750*time.Millisecond, func(time.Time) tea.Msg { return tickStateMsg{} })
}

// previewTickMsg fires on a fixed interval and drives all pane-preview
// captures. Replaces the v0.1 paneCapturedMsg -> tea.Tick -> capturePaneTickMsg
// chain, which deadlocked when the user navigated between fire and response.
type previewTickMsg struct{}

func previewTickCmd(interval time.Duration) tea.Cmd {
	if interval <= 0 {
		interval = 2000 * time.Millisecond
	}
	return tea.Tick(interval, func(time.Time) tea.Msg { return previewTickMsg{} })
}

func capturePaneCmd(c *cli.Ctx, sid string, lines int) tea.Cmd {
	return func() tea.Msg {
		out, _ := c.Tmux.CapturePane(sid, lines)
		return paneCapturedMsg{sid: sid, content: out}
	}
}
