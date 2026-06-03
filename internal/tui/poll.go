// internal/tui/poll.go
package tui

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/dhruvsaxena1998/cleo/internal/cli"
	"github.com/dhruvsaxena1998/cleo/internal/projects"
	"github.com/dhruvsaxena1998/cleo/internal/reconcile"
	"github.com/dhruvsaxena1998/cleo/internal/state"
	"github.com/dhruvsaxena1998/cleo/internal/sys"
)

type stateLoadedMsg struct {
	projects []projects.Project
	sessions []state.Session
}

type tickStateMsg struct{}

type statusExpiredMsg struct {
	id int
}

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

func statusExpiryCmd(id int, d time.Duration) tea.Cmd {
	return tea.Tick(d, func(time.Time) tea.Msg { return statusExpiredMsg{id: id} })
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

// ── Agent memory collection ───────────────────────────────────────────────────

type agentMemTickMsg struct{}
type agentMemMsg struct{ bytes uint64 }

// agentMemTickCmd fires a slow (~2s) tick to collect memory usage of all agent
// processes managed by cleo. This is separate from the main 750ms state tick
// because walking process trees can be expensive with many sessions.
func agentMemTickCmd() tea.Cmd {
	return tea.Tick(2*time.Second, func(time.Time) tea.Msg { return agentMemTickMsg{} })
}

func agentMemCmd(c *cli.Ctx) tea.Cmd {
	return func() tea.Msg {
		var total int64
		sessions, err := c.State.List()
		if err != nil {
			return agentMemMsg{bytes: 0}
		}
		for _, sess := range sessions {
			if sess.State.IsFinished() {
				continue
			}
			pids, err := c.Tmux.SessionPIDs(sess.ID)
			if err != nil {
				continue
			}
			for _, pid := range pids {
				rss, err := sys.ProcessTreeRSS(pid)
				if err != nil {
					continue
				}
				total += rss
			}
		}
		return agentMemMsg{bytes: uint64(total)}
	}
}
