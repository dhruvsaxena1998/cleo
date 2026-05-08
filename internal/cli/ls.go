package cli

import (
	"encoding/json"
	"fmt"
	"sort"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"

	"github.com/dhruvsaxena1998/cleo/internal/reconcile"
)

type lsRow struct {
	Project     string     `json:"project"`
	Agent       *string    `json:"agent"`
	Name        *string    `json:"name"`
	State       *string    `json:"state"`
	ID          *string    `json:"id"`
	StartedAt   *time.Time `json:"started_at"`
	LastEventAt *time.Time `json:"last_event_at"`
}

func strPtr(s string) *string { return &s }

func timePtr(t time.Time) *time.Time {
	if t.IsZero() {
		return nil
	}
	return &t
}

func newLsCmd(getCtx func() *Ctx) *cobra.Command {
	var jsonFlag bool
	cmd := &cobra.Command{
		Use:   "ls",
		Short: "List projects and sessions",
		RunE: func(cmd *cobra.Command, args []string) error {
			c := getCtx()
			_ = reconcile.Run(c.State, c.Tmux, c.Config.Retention.IdleToCompletedTimeout)
			projects, _ := c.Projects.List()
			sessions, _ := c.State.List()

			sort.SliceStable(projects, func(i, j int) bool { return projects[i].ID < projects[j].ID })
			byProj := map[string][]int{}
			for i, s := range sessions {
				byProj[s.ProjectID] = append(byProj[s.ProjectID], i)
			}

			if jsonFlag {
				var rows []lsRow
				for _, p := range projects {
					if len(byProj[p.ID]) == 0 {
						rows = append(rows, lsRow{Project: p.ID})
						continue
					}
					for _, i := range byProj[p.ID] {
						s := sessions[i]
						st := string(s.State)
						rows = append(rows, lsRow{
							Project:     p.ID,
							Agent:       strPtr(s.Agent),
							Name:        strPtr(s.Name),
							State:       strPtr(st),
							ID:          strPtr(s.ID),
							StartedAt:   timePtr(s.StartedAt),
							LastEventAt: timePtr(s.LastEventAt),
						})
					}
				}
				if rows == nil {
					rows = []lsRow{}
				}
				b, err := json.MarshalIndent(rows, "", "  ")
				if err != nil {
					return err
				}
				fmt.Fprintf(cmd.OutOrStdout(), "%s\n", b)
				return nil
			}

			tw := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
			fmt.Fprintln(tw, "PROJECT\tAGENT\tNAME\tSTATE\tID")
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
	cmd.Flags().BoolVar(&jsonFlag, "json", false, "output as JSON array")
	return cmd
}
