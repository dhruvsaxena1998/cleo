package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

const Version = "0.1.0-dev"

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
		newHookCmd(getCtx),
	)
	return root
}

func Execute(tuiRun func(*Ctx) error) {
	if err := NewRootCmd(tuiRun).Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}
