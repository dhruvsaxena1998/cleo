package cli

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/dhruvsaxena1998/cleo/internal/config"
	"github.com/dhruvsaxena1998/cleo/internal/ids"
	"github.com/dhruvsaxena1998/cleo/internal/projects"
	"github.com/dhruvsaxena1998/cleo/internal/state"
	"github.com/dhruvsaxena1998/cleo/internal/tmux"
)

func newRunCmd(getCtx func() *Ctx) *cobra.Command {
	var name string
	var cwdFlag string
	var yes bool

	cmd := &cobra.Command{
		Use:   "run <agent>",
		Short: "Spawn an agent in the current project",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c := getCtx()
			agentName := args[0]
			agent, ok := c.Config.Agents[agentName]
			if !ok {
				return fmt.Errorf("unknown agent %q (configured: %v)", agentName, agentKeys(c.Config.Agents))
			}

			cwd := cwdFlag
			if cwd == "" {
				wd, err := os.Getwd()
				if err != nil {
					return err
				}
				cwd = wd
			}
			cwd, _ = filepath.Abs(cwd)

			proj, err := c.Projects.ResolveFromCwd(cwd)
			if errors.Is(err, projects.ErrNotFound) {
				if !yes {
					fmt.Fprintf(cmd.OutOrStdout(), "register %q as a new project? [Y/n] ", cwd)
					ans, _ := bufio.NewReader(os.Stdin).ReadString('\n')
					ans = strings.TrimSpace(strings.ToLower(ans))
					if ans != "" && ans != "y" && ans != "yes" {
						return errors.New("aborted")
					}
				}
				proj, err = c.Projects.Add(cwd)
				if err != nil {
					return err
				}
				fmt.Fprintf(cmd.OutOrStdout(), "registered project %q\n", proj.ID)
			} else if err != nil {
				return err
			}

			// Compute slug: user name (slugified) or counter
			existing := existingSlugs(c, proj.ID, agentName)
			var slug string
			if name != "" {
				slug = ids.DedupeSlug(ids.Slugify(name), existing)
			} else {
				slug = nextCounterSlug(existing)
			}
			sid := ids.MakeSessionID(proj.ID, agentName, slug)

			sess := state.Session{
				ID:        sid,
				ProjectID: proj.ID,
				Agent:     agentName,
				Name:      slug,
				State:     state.Spawning,
				StartedAt: time.Now().UTC(),
			}
			if err := c.State.Put(sess); err != nil {
				return err
			}
			err = c.Tmux.NewSession(tmux.NewSessionOpts{
				Name: sid,
				Cwd:  proj.Path,
				Cmd:  agent.Command,
				Env:  map[string]string{"CLEO_SESSION_ID": sid},
			})
			if err != nil {
				_ = c.State.Delete(sid)
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "spawned %s — attach with `cleo attach %s`\n", sid, sid)
			return nil
		},
	}
	cmd.Flags().StringVar(&name, "name", "", "session name (slugified)")
	cmd.Flags().StringVar(&cwdFlag, "cwd", "", "override working directory")
	cmd.Flags().BoolVar(&yes, "yes", false, "skip auto-register confirmation")
	return cmd
}

func agentKeys(m map[string]config.Agent) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

func existingSlugs(c *Ctx, project, agent string) map[string]bool {
	out := map[string]bool{}
	all, _ := c.State.List()
	prefix := fmt.Sprintf("cleo-%s-%s-", project, agent)
	for _, s := range all {
		if strings.HasPrefix(s.ID, prefix) {
			out[s.Name] = true
		}
	}
	return out
}

func nextCounterSlug(existing map[string]bool) string {
	for i := 1; ; i++ {
		c := fmt.Sprintf("%d", i)
		if !existing[c] {
			return c
		}
	}
}
