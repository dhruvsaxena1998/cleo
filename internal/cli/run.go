package cli

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"os/exec"
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

			cwd := cwdFlag
			if cwd == "" {
				wd, err := os.Getwd()
				if err != nil {
					return err
				}
				cwd = wd
			}
			cwd, _ = filepath.Abs(cwd)

			lifecycle := sessionlifecycle.New(sessionlifecycle.Options{
				Config:   c.Config,
				Projects: c.Projects,
				State:    c.State,
				Tmux:     c.Tmux,
			})
			result, err := lifecycle.Create(sessionlifecycle.CreateInput{
				Agent:               agentName,
				Name:                name,
				Path:                cwd,
				AutoRegisterProject: yes,
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
				})
			}
			if err != nil {
				return err
			}
			if result.ProjectRegistered {
				fmt.Fprintf(cmd.OutOrStdout(), "registered project %q\n", result.Project.ID)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "spawned %s\n", result.Session.ID)
			if !noAttach {
				markFocused(c, result.Session.ID, true)
				var attachCmd *exec.Cmd
				if os.Getenv("TMUX") != "" {
					attachCmd = exec.Command("tmux", "switch-client", "-t", result.Session.ID)
				} else {
					attachCmd = exec.Command("tmux", "attach-session", "-t", result.Session.ID)
				}
				attachCmd.Stdin = os.Stdin
				attachCmd.Stdout = os.Stdout
				attachCmd.Stderr = os.Stderr
				_ = attachCmd.Run()
				markFocused(c, result.Session.ID, false)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&name, "name", "", "session name (slugified)")
	cmd.Flags().StringVar(&cwdFlag, "cwd", "", "override working directory")
	cmd.Flags().BoolVar(&yes, "yes", false, "skip auto-register confirmation")
	cmd.Flags().BoolVar(&noAttach, "no-attach", false, "spawn without attaching to the session")
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
