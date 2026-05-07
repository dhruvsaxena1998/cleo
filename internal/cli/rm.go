package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newRmCmd(getCtx func() *Ctx) *cobra.Command {
	return &cobra.Command{
		Use:   "rm <project>",
		Short: "Unregister a project (running sessions keep running)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c := getCtx()
			if err := c.Projects.Remove(args[0]); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "removed project %q\n", args[0])
			return nil
		},
	}
}
