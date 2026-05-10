package cli

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"

	"github.com/dhruvsaxena1998/cleo/internal/hooks"
)

func newDoctorCmd(getCtx func() *Ctx) *cobra.Command {
	return &cobra.Command{
		Use:   "doctor",
		Short: "Check Cleo hook setup",
		RunE: func(cmd *cobra.Command, args []string) error {
			c := getCtx()
			home, _ := os.UserHomeDir()
			report := diagnoseHooks(
				filepath.Join(home, ".claude", "settings.json"),
				filepath.Join(home, ".codex", "hooks.json"),
				filepath.Join(home, ".codex", "config.toml"),
				c.Paths.HookTraceLog(),
			)
			printDoctorReport(cmd.OutOrStdout(), report)
			return nil
		},
	}
}

type doctorReport struct {
	Checks []doctorCheck
}

type doctorCheck struct {
	Label  string
	OK     bool
	Detail string
}

func diagnoseHooks(claudeSettingsPath, codexHooksPath, codexConfigPath, hookTracePath string) doctorReport {
	return doctorReport{Checks: []doctorCheck{
		checkClaudeHooks(claudeSettingsPath),
		checkCodexFeatureFlag(codexConfigPath),
		checkCodexHooks(codexHooksPath),
		checkHookTrace(hookTracePath, "claude"),
		checkHookTrace(hookTracePath, "codex"),
	}}
}

func checkClaudeHooks(path string) doctorCheck {
	b, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return doctorCheck{Label: "Claude hooks", Detail: fmt.Sprintf("missing %s; run cleo init", path)}
	}
	if err != nil {
		return doctorCheck{Label: "Claude hooks", Detail: err.Error()}
	}
	var settings map[string]any
	if err := json.Unmarshal(b, &settings); err != nil {
		return doctorCheck{Label: "Claude hooks", Detail: fmt.Sprintf("invalid JSON in %s: %v", path, err)}
	}
	configured, _ := settings["hooks"].(map[string]any)
	missing := missingHookEvents(configured, hooks.ClaudeEvents(), "hook claude")
	if len(missing) > 0 {
		return doctorCheck{Label: "Claude hooks", Detail: fmt.Sprintf("missing Cleo command for %s in %s; run cleo init", strings.Join(missing, ", "), path)}
	}
	return doctorCheck{Label: "Claude hooks", OK: true, Detail: fmt.Sprintf("%d hooks installed", len(hooks.ClaudeEvents()))}
}

func checkCodexFeatureFlag(path string) doctorCheck {
	b, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return doctorCheck{Label: "Codex feature flag", Detail: fmt.Sprintf("missing %s; run cleo init", path)}
	}
	if err != nil {
		return doctorCheck{Label: "Codex feature flag", Detail: err.Error()}
	}
	content := string(b)
	if strings.Contains(content, "codex_hooks") {
		return doctorCheck{Label: "Codex feature flag", Detail: fmt.Sprintf("deprecated codex_hooks flag found in %s; run cleo init", path)}
	}
	if !strings.Contains(content, "hooks = true") {
		return doctorCheck{Label: "Codex feature flag", Detail: fmt.Sprintf("[features].hooks = true not found in %s; run cleo init", path)}
	}
	return doctorCheck{Label: "Codex feature flag", OK: true, Detail: "[features].hooks = true"}
}

func checkCodexHooks(path string) doctorCheck {
	b, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return doctorCheck{Label: "Codex hooks", Detail: fmt.Sprintf("missing %s; run cleo init", path)}
	}
	if err != nil {
		return doctorCheck{Label: "Codex hooks", Detail: err.Error()}
	}
	var settings map[string]any
	if err := json.Unmarshal(b, &settings); err != nil {
		return doctorCheck{Label: "Codex hooks", Detail: fmt.Sprintf("invalid JSON in %s: %v", path, err)}
	}
	configured, _ := settings["hooks"].(map[string]any)
	missing := missingHookEvents(configured, hooks.CodexEvents(), "hook codex")
	if len(missing) > 0 {
		return doctorCheck{Label: "Codex hooks", Detail: fmt.Sprintf("missing Cleo command for %s in %s; run cleo init", strings.Join(missing, ", "), path)}
	}
	return doctorCheck{Label: "Codex hooks", OK: true, Detail: fmt.Sprintf("%d hooks installed", len(hooks.CodexEvents()))}
}

func checkHookTrace(path, protocol string) doctorCheck {
	label := fmt.Sprintf("%s hook activity", protocolTitle(protocol))
	last, err := lastHookTrace(path, protocol)
	if errors.Is(err, os.ErrNotExist) {
		return doctorCheck{Label: label, Detail: fmt.Sprintf("no %s hook trace yet at %s; run a small %s task", protocol, path, protocolTitle(protocol))}
	}
	if err != nil {
		return doctorCheck{Label: label, Detail: err.Error()}
	}
	if last.Result != "resolved" {
		return doctorCheck{Label: label, Detail: fmt.Sprintf("last hook %s was %s (cwd=%q); Cleo could not map it to a session", last.Event, last.Result, last.Cwd)}
	}
	source := "cwd fallback"
	if last.EnvSession {
		source = "CLEO_SESSION_ID"
	}
	return doctorCheck{Label: label, OK: true, Detail: fmt.Sprintf("last hook %s resolved to %s via %s", last.Event, last.ResolvedSession, source)}
}

type hookTraceRow struct {
	At              string `json:"at"`
	Protocol        string `json:"protocol"`
	Event           string `json:"event"`
	Cwd             string `json:"cwd"`
	EnvSession      bool   `json:"env_session"`
	ResolvedSession string `json:"resolved_session"`
	Result          string `json:"result"`
	FallbackReason  string `json:"fallback_reason"`
}

func lastHookTrace(path, protocol string) (hookTraceRow, error) {
	f, err := os.Open(path)
	if err != nil {
		return hookTraceRow{}, err
	}
	defer f.Close()

	var last hookTraceRow
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		var row hookTraceRow
		if err := json.Unmarshal(scanner.Bytes(), &row); err != nil {
			continue
		}
		if row.Protocol == protocol {
			last = row
		}
	}
	if err := scanner.Err(); err != nil {
		return hookTraceRow{}, err
	}
	if last.Protocol == "" {
		return hookTraceRow{}, os.ErrNotExist
	}
	return last, nil
}

func protocolTitle(protocol string) string {
	switch protocol {
	case "codex":
		return "Codex"
	case "claude":
		return "Claude"
	default:
		return protocol
	}
}

func missingHookEvents(configured map[string]any, expected []string, commandNeedle string) []string {
	var missing []string
	for _, event := range expected {
		entry, ok := configured[event]
		if !ok || !hookEntryHasCommand(entry, commandNeedle, event) {
			missing = append(missing, event)
		}
	}
	return missing
}

func hookEntryHasCommand(entry any, commandNeedle, event string) bool {
	b, err := json.Marshal(entry)
	if err != nil {
		return false
	}
	text := string(b)
	return strings.Contains(text, commandNeedle) && strings.Contains(text, event)
}

var (
	okStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("#a6e3a1"))
	warnStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#f38ba8"))
)

func printDoctorReport(w io.Writer, report doctorReport) {
	fmt.Fprintln(w, "Cleo doctor")
	fmt.Fprintln(w)
	for _, check := range report.Checks {
		var symbol string
		if check.OK {
			symbol = okStyle.Render("✓")
		} else {
			symbol = warnStyle.Render("✗")
		}
		fmt.Fprintf(w, "%s %s: %s\n", symbol, check.Label, check.Detail)
	}
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Codex approval check:")
	fmt.Fprintln(w, "  Cleo can verify installed files, but Codex keeps hook approval state internally.")
	fmt.Fprintln(w, "  If Codex shows hooks under Review, run /hooks inside Codex and approve these hook names:")
	for _, event := range hooks.CodexEvents() {
		fmt.Fprintf(w, "    - %s\n", event)
	}
	fmt.Fprintln(w, "  Do not run hook commands manually; Codex runs them after approval.")
}
