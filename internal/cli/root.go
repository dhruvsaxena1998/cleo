package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

// Version is the user-facing version string. Release builds override this
// via -ldflags "-X github.com/dhruvsaxena1998/cleo/internal/cli.Version=...".
var Version = "0.1.0-alpha.1"

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
		newRunCmd(getCtx),
		newAttachCmd(getCtx),
		newKillCmd(getCtx),
		newPruneCmd(getCtx),
		newInitCmd(getCtx),
		newCleanupCmd(getCtx),
		newDoctorCmd(getCtx),
		newHookCmd(getCtx),
		newFocusCmd(getCtx),
		newRenameCmd(getCtx),
	)
	return root
}

func Execute(tuiRun func(*Ctx) error) {
	if err := NewRootCmd(tuiRun).Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}
