package cli

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/dhruvsaxena1998/cleo/internal/state"
	"github.com/spf13/cobra"
)

func newKillCmd(getCtx func() *Ctx) *cobra.Command {
	var yes bool
	cmd := &cobra.Command{
		Use:   "kill <session-id>",
		Short: "Kill a running session",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id := args[0]
			c := getCtx()
			if _, err := c.State.Get(id); err != nil {
				if errors.Is(err, state.ErrSessionNotFound) {
					return fmt.Errorf("session %q not found", id)
				}
				return err
			}
			if !yes {
				fmt.Fprintf(cmd.OutOrStdout(), "kill %q? [y/N] ", id)
				ans, _ := bufio.NewReader(os.Stdin).ReadString('\n')
				if strings.TrimSpace(strings.ToLower(ans)) != "y" {
					return errors.New("aborted")
				}
			}
			if err := c.Tmux.Kill(id); err != nil {
				fmt.Fprintf(cmd.ErrOrStderr(), "warning: tmux kill failed: %v\n", err)
			}
			return c.State.Delete(id)
		},
	}
	cmd.Flags().BoolVar(&yes, "yes", false, "skip confirmation")
	return cmd
}
