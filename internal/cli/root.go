package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

// Version is the user-facing version string. Release builds override this
// via -ldflags "-X github.com/dhruvsaxena1998/cleo/internal/cli.Version=...".
var Version = "0.2.0"

func NewRootCmd(tuiRun func(*Ctx) error) *cobra.Command {
	root := &cobra.Command{
		Use:           "cleo",
		Short:         "Terminal session manager for AI coding agents",
		SilenceUsage:  true,
		SilenceErrors: true,
		Version:       Version,
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := NewCtx()
			if err != nil {
				return err
			}
			return tuiRun(c)
		},
	}
	getCtx := func() *Ctx {
		c, err := NewCtx()
		if err != nil {
			fmt.Fprintln(os.Stderr, "error:", err)
			os.Exit(1)
		}
		return c
	}
	root.AddCommand(
		newAddCmd(getCtx),
		newRmCmd(getCtx),
		newLsCmd(getCtx),
		newServeCmd(getCtx),
		newRunCmd(getCtx),
		newAttachCmd(getCtx),
		newKillCmd(getCtx),
		newPruneCmd(getCtx),
		newHooksCmd(getCtx),
		newDoctorCmd(getCtx),
		newFocusCmd(getCtx),
		newRenameCmd(getCtx),
		newEventsCmd(getCtx),
	)
	return root
}

func Execute(tuiRun func(*Ctx) error) {
	if err := NewRootCmd(tuiRun).Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}
