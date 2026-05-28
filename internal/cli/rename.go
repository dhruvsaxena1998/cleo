package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newRenameCmd(getCtx func() *Ctx) *cobra.Command {
	return &cobra.Command{
		Use:   "rename <session-id> <new-name>",
		Short: "Rename a session (updates name only; tmux session is not renamed)",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			c := getCtx()
			lifecycle := c.NewLifecycle()
			result, err := lifecycle.Rename(args[0], args[1])
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "renamed %s: %s → %s\n", result.SessionID, result.OldName, result.NewName)
			return nil
		},
	}
}
