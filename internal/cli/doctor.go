package cli

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"

	"github.com/dhruvsaxena1998/cleo/internal/hooks"
)

func newDoctorCmd(getCtx func() *Ctx) *cobra.Command {
	var quiet bool
	cmd := &cobra.Command{
		Use:           "doctor",
		Short:         "Check Cleo hook setup",
		SilenceUsage:  true, // doctor reports problems; cobra's usage banner is noise on failure
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			c := getCtx()
			report := diagnoseHooks(c.Paths.HookTraceLog())
			if exe, err := os.Executable(); err == nil {
				report.CleoBin = exe
			}
			report.ConfigWarnings = c.Config.Warnings
			analysis := analyzeReport(report)
			printDoctorReportOpts(cmd.OutOrStdout(), report, analysis, doctorPrintOpts{Quiet: quiet})
			if quiet && analysis.HasFailures(report) {
				os.Exit(1)
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&quiet, "quiet", false, "only print failures and non-empty diagnostic sections")
	return cmd
}

type doctorReport struct {
	Checks             []doctorCheck
	HookTracePath      string
	ClaudeSettingsPath string
	CodexHooksPath     string
	CleoBin            string
	ConfigWarnings     []string
}

type doctorCheck struct {
	Label    string
	OK       bool
	Detail   string
	Protocol string // agent name (claude/codex/pi/opencode) or "" — attaches trace inline
}

// diagnoseHooks builds the doctor report by iterating the agent-protocol
// registry: each protocol self-checks its config (Diagnose) and contributes a
// uniform hook-activity check keyed by its name. Adding an agent needs no edit
// here. The claude/codex settings paths are surfaced for the +/= config diff.
func diagnoseHooks(hookTracePath string) doctorReport {
	var checks []doctorCheck
	for _, p := range hooks.Protocols() {
		for _, c := range p.Diagnose() {
			checks = append(checks, doctorCheck{Label: c.Label, OK: c.OK, Detail: c.Detail, Protocol: p.Name()})
		}
		act := checkHookTrace(hookTracePath, p.Name())
		act.Protocol = p.Name()
		checks = append(checks, act)
	}
	report := doctorReport{
		Checks:        checks,
		HookTracePath: hookTracePath,
	}
	if p, ok := hooks.ProtocolByName("claude"); ok {
		report.ClaudeSettingsPath = locationPath(p, "hooks")
	}
	if p, ok := hooks.ProtocolByName("codex"); ok {
		report.CodexHooksPath = locationPath(p, "hooks")
	}
	return report
}

// locationPath returns the path of the protocol's location with the given
// label, or "" if it has none.
func locationPath(p hooks.Protocol, label string) string {
	for _, loc := range p.Locations() {
		if loc.Label == label {
			return loc.Path
		}
	}
	return ""
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

// recentHookTraces returns the n most recent trace rows for the given protocol,
// ordered most-recent-first.
func recentHookTraces(path, protocol string, n int) []hookTraceRow {
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()
	var rows []hookTraceRow
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		var row hookTraceRow
		if err := json.Unmarshal(scanner.Bytes(), &row); err != nil {
			continue
		}
		if row.Protocol == protocol {
			rows = append(rows, row)
		}
	}
	// Reverse to most-recent-first; truncate to n
	for i, j := 0, len(rows)-1; i < j; i, j = i+1, j-1 {
		rows[i], rows[j] = rows[j], rows[i]
	}
	if len(rows) > n {
		rows = rows[:n]
	}
	return rows
}

// attributionFailures returns trace rows whose fallback_reason indicates
// resolution did not succeed (no_match or env_unknown_session). If `since`
// is non-zero, only rows newer than `since` are returned.
func attributionFailures(path string, since time.Time) []hookTraceRow {
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()
	var out []hookTraceRow
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		var row hookTraceRow
		if err := json.Unmarshal(scanner.Bytes(), &row); err != nil {
			continue
		}
		if row.FallbackReason != "no_match" && row.FallbackReason != "env_unknown_session" {
			continue
		}
		if !since.IsZero() {
			if t, err := time.Parse(time.RFC3339, row.At); err == nil && t.Before(since) {
				continue
			}
		}
		out = append(out, row)
	}
	return out
}

// printHookDiffSection renders the per-agent +/= diff. agentLabel is the
// human-readable name ("Claude hooks" / "Codex hooks"); protocol is used in
// the printed command (`cleo hooks invoke claude` etc.). Empty diffs print
// "<agentLabel>: in sync ✓".
func printHookDiffSection(w io.Writer, agentLabel, settingsPath string, d hookDiff, protocol string) {
	if len(d.toAdd) == 0 && len(d.conflicts) == 0 {
		fmt.Fprintf(w, "%s: in sync %s\n", agentLabel, okStyle.Render("✓"))
		return
	}
	fmt.Fprintf(w, "%s (%s):\n", agentLabel, settingsPath)
	for _, ev := range d.matched {
		fmt.Fprintf(w, "  = %-18s cleo hooks invoke %s\n", ev, protocol)
	}
	for _, ev := range d.toAdd {
		fmt.Fprintf(w, "  + %-18s cleo hooks invoke %s    (would install)\n", ev, protocol)
	}
	for _, ev := range d.conflicts {
		fmt.Fprintf(w, "  - %-18s cleo hooks invoke %s    (foreign or divergent entry present)\n", ev, protocol)
	}
}

type hookDiff struct {
	matched   []string
	toAdd     []string
	conflicts []string // entries that exist but don't match cleo's expected command
}

// hookConfigDiff compares the on-disk settings file at settingsPath against
// the entries cleo would install (expectedEntries: keyed by event name).
// Returns the per-event matched / toAdd / conflicts buckets, alphabetised.
// If settingsPath is missing or unparseable, every expected entry is treated
// as toAdd.
func hookConfigDiff(settingsPath string, expectedEntries map[string]any) hookDiff {
	var d hookDiff
	b, err := os.ReadFile(settingsPath)
	if err != nil {
		for k := range expectedEntries {
			d.toAdd = append(d.toAdd, k)
		}
		sort.Strings(d.toAdd)
		return d
	}
	var settings map[string]any
	if err := json.Unmarshal(b, &settings); err != nil {
		for k := range expectedEntries {
			d.toAdd = append(d.toAdd, k)
		}
		sort.Strings(d.toAdd)
		return d
	}
	configured, _ := settings["hooks"].(map[string]any)
	for event, expected := range expectedEntries {
		actual, ok := configured[event]
		if !ok {
			d.toAdd = append(d.toAdd, event)
			continue
		}
		eb, _ := json.Marshal(expected)
		ab, _ := json.Marshal(actual)
		if string(eb) == string(ab) {
			d.matched = append(d.matched, event)
		} else {
			d.conflicts = append(d.conflicts, event)
		}
	}
	sort.Strings(d.matched)
	sort.Strings(d.toAdd)
	sort.Strings(d.conflicts)
	return d
}

// truncRight truncates s to at most n display columns, appending an ellipsis
// when truncation occurs. Naive byte-based truncation; safe for ASCII session
// IDs and event labels.
func truncRight(s string, n int) string {
	if len(s) <= n {
		return s
	}
	if n <= 0 {
		return ""
	}
	return s[:n-1] + "…"
}

func protocolTitle(protocol string) string {
	switch protocol {
	case "codex":
		return "Codex"
	case "claude":
		return "Claude"
	case "opencode":
		return "OpenCode"
	case "pi":
		return "Pi"
	default:
		return protocol
	}
}

var (
	okStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("#a6e3a1"))
	warnStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#f38ba8"))
)

type doctorPrintOpts struct {
	Quiet bool
}

// doctorAnalysis caches the computed diagnostic data so the printer and the
// failure-detection path don't re-read the trace log + settings files. Build
// it once via analyzeReport and reuse it.
type doctorAnalysis struct {
	Failures   []hookTraceRow // attribution failures in the last 24h
	ClaudeDiff hookDiff       // empty.matched/toAdd/conflicts when CleoBin == ""
	CodexDiff  hookDiff
}

func analyzeReport(report doctorReport) doctorAnalysis {
	a := doctorAnalysis{
		Failures: attributionFailures(report.HookTracePath, time.Now().Add(-24*time.Hour)),
	}
	if report.CleoBin != "" {
		a.ClaudeDiff = hookConfigDiff(report.ClaudeSettingsPath, hooks.ExpectedClaudeEntries(report.CleoBin))
		a.CodexDiff = hookConfigDiff(report.CodexHooksPath, hooks.ExpectedCodexEntries(report.CleoBin))
	}
	return a
}

// HasFailures reports whether the report would warrant a non-zero exit in
// --quiet mode: any failed check, attribution failures, or non-empty diff.
func (a doctorAnalysis) HasFailures(report doctorReport) bool {
	for _, check := range report.Checks {
		if !check.OK {
			return true
		}
	}
	if len(a.Failures) > 0 {
		return true
	}
	if len(a.ClaudeDiff.toAdd)+len(a.ClaudeDiff.conflicts) > 0 {
		return true
	}
	if len(a.CodexDiff.toAdd)+len(a.CodexDiff.conflicts) > 0 {
		return true
	}
	if len(report.ConfigWarnings) > 0 {
		return true
	}
	return false
}

func printDoctorReport(w io.Writer, report doctorReport) {
	printDoctorReportOpts(w, report, analyzeReport(report), doctorPrintOpts{})
}

func printDoctorReportOpts(w io.Writer, report doctorReport, analysis doctorAnalysis, opts doctorPrintOpts) {
	if !opts.Quiet {
		fmt.Fprintln(w, "Cleo doctor")
		fmt.Fprintln(w)
	}
	for _, check := range report.Checks {
		if opts.Quiet && check.OK {
			continue
		}
		var symbol string
		if check.OK {
			symbol = okStyle.Render("✓")
		} else {
			symbol = warnStyle.Render("✗")
		}
		fmt.Fprintf(w, "%s %s: %s\n", symbol, check.Label, check.Detail)
		if !opts.Quiet && strings.Contains(check.Label, "hook activity") && check.Protocol != "" {
			traces := recentHookTraces(report.HookTracePath, check.Protocol, 3)
			if len(traces) > 0 {
				fmt.Fprintln(w, "  Last hook traces:")
				for _, tr := range traces {
					ts := tr.At
					if t, err := time.Parse(time.RFC3339, tr.At); err == nil {
						ts = t.Local().Format("15:04:05")
					}
					fmt.Fprintf(w, "    %s  %-18s %-40s %s\n", ts, tr.Event, truncRight(tr.ResolvedSession, 40), tr.FallbackReason)
				}
			}
		}
	}
	if len(report.ConfigWarnings) > 0 {
		fmt.Fprintln(w)
		fmt.Fprintln(w, "Config warnings:")
		for _, warning := range report.ConfigWarnings {
			fmt.Fprintf(w, "  - %s\n", warning)
		}
	}
	if len(analysis.Failures) > 0 {
		fmt.Fprintln(w)
		fmt.Fprintf(w, "Attribution failures (last 24h): %d\n", len(analysis.Failures))
		fmt.Fprintln(w, "  Last 3:")
		last := analysis.Failures
		if len(last) > 3 {
			last = last[len(last)-3:]
		}
		for _, tr := range last {
			ts := tr.At
			if t, err := time.Parse(time.RFC3339, tr.At); err == nil {
				ts = t.Local().Format("15:04:05")
			}
			cwd := tr.Cwd
			if cwd == "" {
				cwd = "(no cwd)"
			}
			fmt.Fprintf(w, "    %s  %-30s %-18s %s\n", ts, truncRight(cwd, 30), tr.Event, tr.FallbackReason)
		}
	}
	if report.CleoBin != "" {
		claudeSync := len(analysis.ClaudeDiff.toAdd) == 0 && len(analysis.ClaudeDiff.conflicts) == 0
		codexSync := len(analysis.CodexDiff.toAdd) == 0 && len(analysis.CodexDiff.conflicts) == 0
		if !opts.Quiet || !claudeSync {
			fmt.Fprintln(w)
			printHookDiffSection(w, "Claude hooks", report.ClaudeSettingsPath, analysis.ClaudeDiff, "claude")
		}
		if !opts.Quiet || !codexSync {
			fmt.Fprintln(w)
			printHookDiffSection(w, "Codex hooks", report.CodexHooksPath, analysis.CodexDiff, "codex")
		}
	}
	if !opts.Quiet {
		fmt.Fprintln(w)
		fmt.Fprintln(w, "Codex approval check:")
		fmt.Fprintln(w, "  Cleo can verify installed files, but Codex keeps hook approval state internally.")
		fmt.Fprintln(w, "  If Codex shows hooks under Review, run /hooks inside Codex and approve these hook names:")
		for _, event := range hooks.CodexEvents() {
			fmt.Fprintf(w, "    - %s\n", event)
		}
		fmt.Fprintln(w, "  Do not run hook commands manually; Codex runs them after approval.")
	}
}
