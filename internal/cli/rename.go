package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/dhruvsaxena1998/cleo/internal/ids"
)

func newRenameCmd(getCtx func() *Ctx) *cobra.Command {
	return &cobra.Command{
		Use:   "rename <session-id> <new-name>",
		Short: "Rename a session (updates name only; tmux session is not renamed)",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			c := getCtx()
			sess, err := c.State.Get(args[0])
			if err != nil {
				return fmt.Errorf("session %q not found", args[0])
			}
			oldSlug := sess.Name
			newSlug := ids.Slugify(args[1])
			sess.Name = newSlug
			if err := c.State.Put(sess); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "renamed %s: %s → %s\n", sess.ID, oldSlug, newSlug)
			return nil
		},
	}
}
