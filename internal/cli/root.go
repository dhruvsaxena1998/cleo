package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

const Version = "0.1.0-dev"

func NewRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:           "cleo",
		Short:         "Terminal session manager for AI coding agents",
		SilenceUsage:  true,
		SilenceErrors: true,
		Version:       Version,
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Println("cleo TUI — coming in phase 9")
			return nil
		},
	}
	getCtx := func() *Ctx {
		c, err := NewCtx()
		if err != nil {
			panic(err)
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

func Execute() {
	if err := NewRootCmd().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}
