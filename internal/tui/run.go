package tui

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/dhruvsaxena1998/cleo/internal/cli"
)

func Run(c *cli.Ctx) error {
	m := New(c)

	// Sync the terminal background colour to the theme's Base so that areas
	// not covered by TUI content (below the footer, ANSI-reset gaps) use the
	// same colour instead of the terminal's own configured background, which
	// would clash with every theme except one that happens to match.
	// \x1b]11;colour\x07  — OSC 11:  set background colour
	// \x1b]111\x07        — OSC 111: reset background to terminal default on exit
	fmt.Fprintf(os.Stdout, "\x1b]11;%s\x07", m.theme.Base)
	defer fmt.Fprintf(os.Stdout, "\x1b]111\x07")

	_, err := tea.NewProgram(m, tea.WithAltScreen(), tea.WithMouseCellMotion()).Run()
	return err
}
