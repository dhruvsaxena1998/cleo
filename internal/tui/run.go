package tui

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/dhruvsaxena1998/cleo/internal/cli"
)

func Run(c *cli.Ctx) error {
	m := New(c)

	// Sync the terminal background colour to the theme's Base so that areas
	// not covered by TUI content (below the footer, ANSI-reset gaps) use the
	// same colour instead of the terminal's own configured background, which
	// would clash with every theme except one that happens to match.
	syncTerminalBackground(m.theme.Base)
	// OSC 111 resets the background to the terminal default on exit.
	defer fmt.Fprint(os.Stdout, "\x1b]111\x07")

	_, err := tea.NewProgram(m, tea.WithAltScreen(), tea.WithMouseCellMotion()).Run()
	return err
}

// syncTerminalBackground emits OSC 11 to set the terminal's background colour.
// Called at startup and again whenever the theme changes at runtime (via
// setBackgroundCmd) so the area outside TUI content tracks the active theme
// without a restart.
func syncTerminalBackground(base lipgloss.Color) {
	fmt.Fprintf(os.Stdout, "\x1b]11;%s\x07", base)
}

// setBackgroundCmd re-syncs the terminal background from inside the update loop.
// The renderer writes to the same stdout on its own goroutine, but OSC 11 only
// sets a colour (no cursor moves or cell writes), so an interleave is harmless.
func setBackgroundCmd(base lipgloss.Color) tea.Cmd {
	return func() tea.Msg {
		syncTerminalBackground(base)
		return nil
	}
}
