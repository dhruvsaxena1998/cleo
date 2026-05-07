package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/dhruvsaxena1998/cleo/internal/hooks"
	"github.com/dhruvsaxena1998/cleo/internal/sound"
)

func newInitCmd(getCtx func() *Ctx) *cobra.Command {
	return &cobra.Command{
		Use:   "init",
		Short: "Install hooks into ~/.claude/settings.json (and codex)",
		RunE: func(cmd *cobra.Command, args []string) error {
			c := getCtx()
			if err := sound.ExtractDefaults(c.Paths.SoundsDir()); err != nil {
				return err
			}
			cleoBin, err := os.Executable()
			if err != nil {
				return err
			}
			cleoBin, _ = filepath.Abs(cleoBin)
			home, _ := os.UserHomeDir()
			claudeSettings := filepath.Join(home, ".claude", "settings.json")
			if err := hooks.InstallClaude(claudeSettings, cleoBin); err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), "installed claude hooks at", claudeSettings)
			// Codex install: TODO once event names confirmed; placeholder note.
			fmt.Fprintln(cmd.OutOrStdout(), "codex hook install: pending — see spec §17")
			return nil
		},
	}
}
