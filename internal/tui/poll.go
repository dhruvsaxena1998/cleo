package tui

import (
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

func loadStateCmd(c *cli.Ctx) tea.Cmd {
	return func() tea.Msg { return stateLoadedMsg{} }
}

func tickStateCmd() tea.Cmd {
	return func() tea.Msg { return tickStateMsg{} }
}
