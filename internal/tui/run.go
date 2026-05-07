package tui

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/dhruvsaxena1998/cleo/internal/cli"
)

func Run(c *cli.Ctx) error {
	_, err := tea.NewProgram(New(c), tea.WithAltScreen(), tea.WithMouseCellMotion()).Run()
	return err
}
