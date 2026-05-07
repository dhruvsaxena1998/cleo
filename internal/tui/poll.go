// internal/tui/poll.go
package tui

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/dhruvsaxena1998/cleo/internal/cli"
	"github.com/dhruvsaxena1998/cleo/internal/projects"
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
		ps, _ := c.Projects.List()
		ss, _ := c.State.List()
		return stateLoadedMsg{projects: ps, sessions: ss}
	}
}

func tickStateCmd() tea.Cmd {
	return tea.Tick(750*time.Millisecond, func(time.Time) tea.Msg { return tickStateMsg{} })
}

type capturePaneTickMsg struct{ sid string }

func capturePaneCmd(c *cli.Ctx, sid string, lines int) tea.Cmd {
	return func() tea.Msg {
		out, _ := c.Tmux.CapturePane(sid, lines)
		return paneCapturedMsg{sid: sid, content: out}
	}
}
