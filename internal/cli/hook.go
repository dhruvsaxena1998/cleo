package cli

import (
	"os"

	"github.com/spf13/cobra"

	"github.com/dhruvsaxena1998/cleo/internal/hooks"
)

func newHookCmd(getCtx func() *Ctx) *cobra.Command {
	return &cobra.Command{
		Use:    "hook <protocol> <event>",
		Short:  "Internal: invoked by hook configs",
		Args:   cobra.ExactArgs(2),
		Hidden: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			c := getCtx()
			deps := hooks.Deps{
				Paths:  c.Paths,
				State:  c.State,
				Config: c.Config,
				Events: c.Events,
				Sound:  c.Player,
				Now:    hooks.DefaultNow,
			}
			return hooks.Handle(deps, args[0], args[1], os.Stdin, cmd.OutOrStdout())
		},
	}
}
