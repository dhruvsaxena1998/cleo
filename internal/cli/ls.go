package cli

import (
	"fmt"
	"sort"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/dhruvsaxena1998/cleo/internal/reconcile"
)

func newLsCmd(getCtx func() *Ctx) *cobra.Command {
	return &cobra.Command{
		Use:   "ls",
		Short: "List projects and sessions",
		RunE: func(cmd *cobra.Command, args []string) error {
			c := getCtx()
			_ = reconcile.Run(c.State, c.Tmux, c.Config.Retention.IdleToCompletedTimeout)
			projects, _ := c.Projects.List()
			sessions, _ := c.State.List()

			tw := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
			fmt.Fprintln(tw, "PROJECT\tAGENT\tNAME\tSTATE\tID")
			sort.SliceStable(projects, func(i, j int) bool { return projects[i].ID < projects[j].ID })
			byProj := map[string][]int{}
			for i, s := range sessions {
				byProj[s.ProjectID] = append(byProj[s.ProjectID], i)
			}
			for _, p := range projects {
				if len(byProj[p.ID]) == 0 {
					fmt.Fprintf(tw, "%s\t-\t-\t-\t-\n", p.ID)
					continue
				}
				for _, i := range byProj[p.ID] {
					s := sessions[i]
					fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\n", p.ID, s.Agent, s.Name, s.State, s.ID)
				}
			}
			return tw.Flush()
		},
	}
}
