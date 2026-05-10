package cli

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/charmbracelet/huh"
	"github.com/spf13/cobra"

	"github.com/dhruvsaxena1998/cleo/internal/hooks"
	"github.com/dhruvsaxena1998/cleo/internal/sound"
)

const (
	hookClaude = "claude"
	hookCodex  = "codex"
	hookPi     = "pi"
)

func newInitCmd(getCtx func() *Ctx) *cobra.Command {
	var force bool
	var yes bool

	cmd := &cobra.Command{
		Use:   "init",
		Short: "Install hooks into agent config files",
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

			selected := []string{hookClaude, hookCodex}
			if !yes {
				if err := promptHookSelection(&selected); err != nil {
					return err
				}
			}

			var results []initInstallResult
			for _, h := range selected {
				switch h {
				case hookClaude:
					path := filepath.Join(home, ".claude", "settings.json")
					if err := hooks.InstallClaude(path, cleoBin, force); err != nil {
						return err
					}
					results = append(results, initInstallResult{
						Name: "Claude Code",
						Files: []string{
							fmt.Sprintf("hooks: %s", path),
						},
						InstalledHooks: hooks.ClaudeEvents(),
					})
				case hookCodex:
					hooksPath := filepath.Join(home, ".codex", "hooks.json")
					configPath := filepath.Join(home, ".codex", "config.toml")
					if err := hooks.InstallCodex(hooksPath, configPath, cleoBin, force); err != nil {
						return err
					}
					results = append(results, initInstallResult{
						Name: "Codex",
						Files: []string{
							fmt.Sprintf("hooks: %s", hooksPath),
							fmt.Sprintf("feature flag: %s ([features].hooks = true)", configPath),
						},
						InstalledHooks:   hooks.CodexEvents(),
						NeedsCodexReview: true,
						ReviewHooks:      hooks.CodexEvents(),
						ReviewCommand:    fmt.Sprintf("%s hook codex", cleoBin),
					})
				case hookPi:
					if err := (hooks.PiProtocol{}).Install(cleoBin, force); err != nil {
						return err
					}
					results = append(results, initInstallResult{
						Name: "Pi",
						Files: []string{
							fmt.Sprintf("extension: %s", filepath.Join(home, ".pi", "agent", "extensions", "cleo.ts")),
						},
						InstalledHooks: hooks.PiEvents(),
					})
				}
			}
			printInitSummary(cmd.OutOrStdout(), results)
			return nil
		},
	}
	cmd.Flags().BoolVar(&force, "force", false, "overwrite conflicting hooks")
	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "install all supported hook systems without prompting")
	return cmd
}

type initInstallResult struct {
	Name             string
	Files            []string
	InstalledHooks   []string
	NeedsCodexReview bool
	ReviewHooks      []string
	ReviewCommand    string
}

func printInitSummary(w io.Writer, results []initInstallResult) {
	fmt.Fprintln(w, "Cleo hooks initialized")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Installed:")
	for _, result := range results {
		fmt.Fprintf(w, "  - %s\n", result.Name)
		for _, file := range result.Files {
			fmt.Fprintf(w, "    %s\n", file)
		}
		if len(result.InstalledHooks) > 0 {
			fmt.Fprintln(w, "    events:")
			for _, hook := range result.InstalledHooks {
				fmt.Fprintf(w, "      - %s\n", hook)
			}
		}
	}

	if hasCodexReviewStep(results) {
		fmt.Fprintln(w)
		fmt.Fprintln(w, "Codex /hooks approval:")
		fmt.Fprintln(w, "  Hook names to approve:")
		for _, result := range results {
			if !result.NeedsCodexReview {
				continue
			}
			for _, hook := range result.ReviewHooks {
				fmt.Fprintf(w, "    - %s\n", hook)
			}
			if result.ReviewCommand != "" {
				fmt.Fprintf(w, "  Match command text starting with: %s\n", result.ReviewCommand)
			}
		}
		fmt.Fprintln(w, "  Do not run these hook commands manually; Codex runs them after approval.")
		fmt.Fprintln(w)
		fmt.Fprintln(w, "Next steps:")
		fmt.Fprintln(w, "  - Restart any open Codex sessions so they reload ~/.codex/config.toml.")
		fmt.Fprintln(w, "  - In Codex, run /hooks and approve the Cleo entries listed above if they appear under Review.")
		fmt.Fprintln(w, "  - Codex will not execute newly installed hooks until this review step is completed.")
	}
}

func hasCodexReviewStep(results []initInstallResult) bool {
	for _, result := range results {
		if result.NeedsCodexReview {
			return true
		}
	}
	return false
}

func promptHookSelection(selected *[]string) error {
	return huh.NewForm(
		huh.NewGroup(
			huh.NewMultiSelect[string]().
				Title("Which hook systems would you like to install?").
				Options(
					huh.NewOption("Claude Code  (~/.claude/settings.json)", hookClaude),
					huh.NewOption("Codex        (~/.codex/hooks.json)", hookCodex),
					huh.NewOption("Pi           (~/.pi/agent/extensions/cleo.ts)", hookPi),
				).
				Value(selected),
		),
	).Run()
}
