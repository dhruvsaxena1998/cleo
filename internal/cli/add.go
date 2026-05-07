package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

func newAddCmd(getCtx func() *Ctx) *cobra.Command {
	return &cobra.Command{
		Use:   "add [path]",
		Short: "Register a project",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			path := "."
			if len(args) == 1 {
				path = args[0]
			}
			abs, err := os.Getwd()
			if err == nil && path == "." {
				path = abs
			}
			c := getCtx()
			p, err := c.Projects.Add(path)
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "registered project %q at %s\n", p.ID, p.Path)
			return nil
		},
	}
}
