package hooks

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/dhruvsaxena1998/cleo/internal/state"
)

// testHomeDir overrides the user's home directory in tests. Every protocol
// derives the config files it owns from homeDir(), so a single override roots
// the whole agent-config tree (~/.claude, ~/.codex, ~/.pi, ~/.config/opencode)
// under a temp dir.
var testHomeDir string

func homeDir() string {
	if testHomeDir != "" {
		return testHomeDir
	}
	h, _ := os.UserHomeDir()
	return h
}

// Location names one config file a protocol owns: a short human label
// ("hooks", "feature flag", "extension", "plugin") and its absolute path.
// Locations() is the source of truth for the prompt hint, the init summary,
// and doctor's "not found at <path>" references.
type Location struct {
	Label string
	Path  string
}

// ReviewStep describes a manual-approval follow-up an Install requires before
// its hooks take effect. Non-nil only for agents (Codex) that gate hooks
// behind in-app approval.
type ReviewStep struct {
	Command string   // the cleo command whose entries must be approved
	Hooks   []string // the hook event names awaiting approval
}

// InstallReport is what Install reports back about a completed install. Files
// written are described by Locations(); the report carries only the dynamic
// follow-up, if any.
type InstallReport struct {
	ManualReview *ReviewStep
}

// Check is one line of a protocol's self-diagnosis: a label, whether it
// passed, and a human detail. Diagnose() returns the config/presence checks;
// the doctor command renders them and attaches trace activity separately.
type Check struct {
	Label  string
	OK     bool
	Detail string
}

// CleanupStatus categorises the outcome of a per-protocol Cleanup call so the
// CLI can render a uniform per-agent summary line.
type CleanupStatus int

const (
	// CleanupStatusMissing — nothing cleo-owned existed on disk for this
	// protocol; the cleanup was a no-op. Rendered as "nothing to remove".
	CleanupStatusMissing CleanupStatus = iota
	// CleanupStatusRemoved — cleo-owned content existed and was removed.
	CleanupStatusRemoved
	// CleanupStatusSkippedModified — an exclusively-cleo file exists on
	// disk but its contents diverge from what cleo would have generated, so
	// it is left untouched. Rendered as "skipped modified".
	CleanupStatusSkippedModified
)

// CleanupOutcome is the structured result of a per-protocol Cleanup call.
type CleanupOutcome struct {
	Status CleanupStatus
	Path   string
	// Notes carries agent-specific caveats the CLI should surface (e.g. Codex
	// leaving its config.toml feature flag in place). Empty for most agents.
	Notes []string
}

// NormalizedEvent is the canonical form every protocol produces after parsing
// its raw payload. Handle() consumes only this — no protocol-specific logic lives
// outside the Protocol implementation.
type NormalizedEvent struct {
	StateEvent       state.Event // empty = no state transition
	SoundEvent       string      // empty = no sound
	Message          string      // Notification / PermissionRequest text
	ToolName         string      // written to the event log
	LogOnly          bool        // log entry only, no state transition (e.g. SubagentStop)
	LogType          string      // events.Entry.Type override when LogOnly=true; defaults to string(StateEvent)
	SuppressWhenIdle bool        // suppress sound when session is already Idle; set by protocols that emit idle-nudge events
}

// Protocol describes a supported agent integration. Implement this interface to
// add a new agent — then add one line to Protocols() below.
type Protocol interface {
	// Name returns the identifier used in "cleo hooks invoke <protocol> <event>".
	Name() string
	// DisplayName returns the human-facing agent name ("Claude Code", "Codex").
	DisplayName() string
	// Events returns the hook event names this protocol subscribes to.
	Events() []string
	// Locations returns the config files this protocol owns, each with a short
	// label. Used by the install prompt, the init summary, and doctor.
	Locations() []Location
	// Install writes hook config into the agent's config file(s) and reports
	// any manual-approval follow-up the install requires.
	Install(cleoBin string, force bool) (InstallReport, error)
	// Cleanup removes cleo-owned hook entries from the agent's config file(s).
	// Returns a structured outcome describing whether cleo content was
	// removed, was absent, or was left in place because the on-disk file
	// has been modified away from cleo's template.
	Cleanup() (CleanupOutcome, error)
	// Diagnose checks the agent's on-disk config and returns one or more health
	// checks (presence/validity). It does not read the trace log — the doctor
	// command attaches per-agent trace activity uniformly via Name().
	Diagnose() []Check
	// Normalize converts a raw event name and JSON payload into a NormalizedEvent.
	// Returns ok=false if the event is unknown and should be silently ignored.
	Normalize(event string, payload []byte) (NormalizedEvent, bool)
	// UsesCwdFallback returns true when the protocol may not propagate
	// CLEO_SESSION_ID to hook subprocesses. resolveSession() calls FindByCwd
	// only when this is true.
	UsesCwdFallback() bool
}

// Protocols returns the registered set of supported agents.
// Adding a new agent: implement Protocol and add one line here.
func Protocols() []Protocol {
	return []Protocol{
		ClaudeProtocol{},
		CodexProtocol{},
		PiProtocol{},
		OpenCodeProtocol{},
	}
}

// installFileTemplate writes template to dir/filename, creating dir. It is
// idempotent (a byte-identical file is left as-is) and refuses to overwrite a
// divergent file unless force. Shared by the pi and opencode Install methods.
func installFileTemplate(dir, filename, template string, force bool) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	dest := filepath.Join(dir, filename)
	existing, err := os.ReadFile(dest)
	if err == nil {
		if string(existing) == template {
			return nil // already up-to-date; idempotent
		}
		if !force {
			return fmt.Errorf("conflict: %s already exists with different content (re-run with --force to overwrite)", dest)
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return os.WriteFile(dest, []byte(template), 0o644)
}

// cleanupFileTemplate removes dir/filename when its contents match template.
// Returns Missing when absent, SkippedModified when the on-disk file diverges
// (left untouched), Removed when deleted. Shared by pi and opencode Cleanup.
func cleanupFileTemplate(dir, filename, template string) (CleanupOutcome, error) {
	dest := filepath.Join(dir, filename)
	content, err := os.ReadFile(dest)
	if errors.Is(err, os.ErrNotExist) {
		return CleanupOutcome{Status: CleanupStatusMissing, Path: dest}, nil
	}
	if err != nil {
		return CleanupOutcome{Path: dest}, err
	}
	if string(content) != template {
		return CleanupOutcome{Status: CleanupStatusSkippedModified, Path: dest}, nil
	}
	if err := os.Remove(dest); err != nil {
		return CleanupOutcome{Path: dest}, err
	}
	return CleanupOutcome{Status: CleanupStatusRemoved, Path: dest}, nil
}

// diagnoseFileTemplate checks a whole-file extension/plugin at path against the
// expected template content. Shared by the pi and opencode Diagnose() methods.
func diagnoseFileTemplate(label, path, template string) Check {
	content, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return Check{Label: label, Detail: fmt.Sprintf("not found at %s — run cleo hooks init to install", path)}
	}
	if err != nil {
		return Check{Label: label, Detail: err.Error()}
	}
	if string(content) != template {
		return Check{Label: label, Detail: fmt.Sprintf("stale — re-run cleo hooks init to update %s", path)}
	}
	return Check{Label: label, OK: true, Detail: path}
}

// ProtocolNames returns the registered protocol names in registration order.
func ProtocolNames() []string {
	protos := Protocols()
	names := make([]string, len(protos))
	for i, p := range protos {
		names[i] = p.Name()
	}
	return names
}

// ProtocolByName looks up a registered protocol by its Name().
func ProtocolByName(name string) (Protocol, bool) {
	return findProtocol(Protocols(), name)
}

func findProtocol(protos []Protocol, name string) (Protocol, bool) {
	for _, p := range protos {
		if p.Name() == name {
			return p, true
		}
	}
	return nil, false
}

var errUnknownProtocol = func(name string) error {
	return fmt.Errorf("unknown protocol %q", name)
}

// Protocol type declarations. Methods are implemented in each protocol's own
// file (claude.go, codex.go, pi.go, opencode.go).
type ClaudeProtocol struct{}
type CodexProtocol struct{}
