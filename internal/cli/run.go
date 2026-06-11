package cli

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/dhruvsaxena1998/cleo/internal/config"
	"github.com/dhruvsaxena1998/cleo/internal/sessionlifecycle"
)

func newRunCmd(getCtx func() *Ctx) *cobra.Command {
	var name string
	var cwdFlag string
	var yes bool
	var noAttach bool
	var worktreeFlag bool
	var noWorktreeFlag bool
	var base string

	cmd := &cobra.Command{
		Use:   "run <agent>",
		Short: "Spawn an agent in the current project",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c := getCtx()
			agentName := args[0]
			_, ok := c.Config.Agents[agentName]
			if !ok {
				return fmt.Errorf("unknown agent %q (configured: %v)", agentName, agentKeys(c.Config.Agents))
			}

			if worktreeFlag && noWorktreeFlag {
				return errors.New("--worktree and --no-worktree are mutually exclusive")
			}
			var worktreeChoice *bool
			if worktreeFlag || noWorktreeFlag {
				worktreeChoice = &worktreeFlag
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

			lifecycle := c.NewLifecycle()
			result, err := lifecycle.Create(sessionlifecycle.CreateInput{
				Agent:               agentName,
				Name:                name,
				Path:                cwd,
				AutoRegisterProject: yes,
				Worktree:            worktreeChoice,
				Base:                base,
			})
			if errors.Is(err, sessionlifecycle.ErrProjectRegistrationNeeded) {
				fmt.Fprintf(cmd.OutOrStdout(), "register %q as a new project? [Y/n] ", cwd)
				ans, _ := bufio.NewReader(os.Stdin).ReadString('\n')
				ans = strings.TrimSpace(strings.ToLower(ans))
				if ans != "" && ans != "y" && ans != "yes" {
					return errors.New("aborted")
				}
				result, err = lifecycle.Create(sessionlifecycle.CreateInput{
					Agent:               agentName,
					Name:                name,
					Path:                cwd,
					AutoRegisterProject: true,
					Worktree:            worktreeChoice,
					Base:                base,
				})
			}
			if err != nil {
				return err
			}
			if result.ProjectRegistered {
				fmt.Fprintf(cmd.OutOrStdout(), "registered project %q\n", result.Project.ID)
			}
			if result.Warning != nil {
				fmt.Fprintf(cmd.ErrOrStderr(), "warning: %v\n", result.Warning)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "spawned %s\n", result.Session.ID)
			if result.Session.HasWorktree() {
				fmt.Fprintf(cmd.OutOrStdout(), "worktree %s on branch %s\n", result.Session.WorktreePath, result.Session.WorktreeBranch)
			}
			if !noAttach {
				plan, err := lifecycle.Attach(result.Session.ID)
				if err != nil {
					return err
				}
				if plan.Cmd != nil {
					plan.Cmd.Stdin = os.Stdin
					plan.Cmd.Stdout = os.Stdout
					plan.Cmd.Stderr = os.Stderr
					_ = plan.Cmd.Run()
					plan.Done()
				}
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&name, "name", "", "session name (slugified)")
	cmd.Flags().StringVar(&cwdFlag, "cwd", "", "override working directory")
	cmd.Flags().BoolVar(&yes, "yes", false, "skip auto-register confirmation")
	cmd.Flags().BoolVar(&noAttach, "no-attach", false, "spawn without attaching to the session")
	cmd.Flags().BoolVar(&worktreeFlag, "worktree", false, "spawn into an isolated git worktree")
	cmd.Flags().BoolVar(&noWorktreeFlag, "no-worktree", false, "spawn in the main working tree even if the project defaults to worktrees")
	cmd.Flags().StringVar(&base, "base", "", "ref the worktree branch starts from (default: current HEAD); requires worktree mode")
	return cmd
}

func agentKeys(m map[string]config.Agent) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
