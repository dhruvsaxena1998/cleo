package hooks

import (
	"fmt"

	"github.com/dhruvsaxena1998/cleo/internal/state"
)

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
	// Name returns the identifier used in "cleo hook <protocol> <event>".
	Name() string
	// Events returns the hook event names this protocol subscribes to.
	Events() []string
	// Install writes hook config into the agent's config file(s).
	Install(cleoBin string, force bool) error
	// Cleanup removes cleo-owned hook entries from the agent's config file(s).
	// Returns a structured outcome describing whether cleo content was
	// removed, was absent, or was left in place because the on-disk file
	// has been modified away from cleo's template.
	Cleanup() (CleanupOutcome, error)
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
