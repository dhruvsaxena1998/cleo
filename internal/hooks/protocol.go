package hooks

import (
	"fmt"

	"github.com/dhruvsaxena1998/cleo/internal/state"
)

// NormalizedEvent is the canonical form every protocol produces after parsing
// its raw payload. Handle() consumes only this — no protocol-specific logic lives
// outside the Protocol implementation.
type NormalizedEvent struct {
	StateEvent state.Event // empty = no state transition
	SoundEvent string      // empty = no sound
	Message    string      // Notification / PermissionRequest text
	ToolName   string      // written to the event log
	LogOnly    bool        // log entry only, no state transition (e.g. SubagentStop)
	LogType    string      // events.Entry.Type override when LogOnly=true; defaults to string(StateEvent)
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
	Cleanup() error
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

// protocolNames returns names for error messages.
func protocolNames(protos []Protocol) []string {
	names := make([]string, len(protos))
	for i, p := range protos {
		names[i] = p.Name()
	}
	return names
}

var errUnknownProtocol = func(name string) error {
	return fmt.Errorf("unknown protocol %q", name)
}

// Protocol type declarations. Methods are implemented in each protocol's own
// file (claude.go, codex.go, pi.go).
type ClaudeProtocol struct{}
type CodexProtocol struct{}
