package cli

import (
	"os"

	"github.com/spf13/cobra"

	"github.com/dhruvsaxena1998/cleo/internal/hooks"
	"github.com/dhruvsaxena1998/cleo/internal/state"
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
				Focused: func(sid string) bool {
					return c.Focus.IsFocused(sid)
				},
				Now: hooks.DefaultNow,
				FindByCwd: func(cwd, agent string) (string, error) {
					proj, err := c.Projects.ResolveFromCwd(cwd)
					if err != nil {
						return "", err
					}
					sessions, err := c.State.List()
					if err != nil {
						return "", err
					}
					// Pick the most recently started active session for this project+agent.
					var best state.Session
					for _, s := range sessions {
						if s.ProjectID == proj.ID && s.Agent == agent && !s.State.IsFinished() {
							if best.ID == "" || s.StartedAt.After(best.StartedAt) {
								best = s
							}
						}
					}
					if best.ID == "" {
						return "", nil
					}
					return best.ID, nil
				},
			}
			return hooks.Handle(deps, args[0], args[1], os.Stdin, cmd.OutOrStdout())
		},
	}
}
