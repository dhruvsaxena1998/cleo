package cli

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/dhruvsaxena1998/cleo/internal/sessionlifecycle"
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
			if !yes {
				fmt.Fprintf(cmd.OutOrStdout(), "kill %q? [y/N] ", id)
				ans, _ := bufio.NewReader(os.Stdin).ReadString('\n')
				if strings.TrimSpace(strings.ToLower(ans)) != "y" {
					return errors.New("aborted")
				}
			}
			lifecycle := sessionlifecycle.New(sessionlifecycle.Options{
				Config:   c.Config,
				Projects: c.Projects,
				State:    c.State,
				Tmux:     c.Tmux,
				Paths:    c.Paths,
				Focus:    c.Focus,
			})
			result, err := lifecycle.Kill(id)
			if err != nil {
				return err
			}
			if result.Warning != nil {
				fmt.Fprintf(cmd.ErrOrStderr(), "warning: tmux kill failed: %v\n", result.Warning)
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&yes, "yes", false, "skip confirmation")
	return cmd
}
