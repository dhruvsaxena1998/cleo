package hooks

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/dhruvsaxena1998/cleo/internal/config"
	"github.com/dhruvsaxena1998/cleo/internal/events"
	"github.com/dhruvsaxena1998/cleo/internal/paths"
	"github.com/dhruvsaxena1998/cleo/internal/state"
)

type Player interface {
	Play(file string) error
	Available() bool
}

type Deps struct {
	Paths  paths.Paths
	State  *state.Store
	Config config.Config
	Events func(sid string) *events.Log
	Sound  Player
	// Now returns the cleo session id from env or returns an error if absent.
	// Test seam.
	Now func() (string, error)
}

func DefaultNow() (string, error) {
	sid := os.Getenv("CLEO_SESSION_ID")
	if sid == "" {
		return "", errNoSession
	}
	return sid, nil
}

var errNoSession = fmt.Errorf("CLEO_SESSION_ID not set")

func Handle(d Deps, protocol, event string, stdin io.Reader, stdout io.Writer) error {
	sid, err := d.Now()
	if err != nil {
		return nil // silent no-op
	}
	body, _ := io.ReadAll(stdin)
	switch protocol {
	case "claude":
		return handleClaude(d, sid, event, body)
	case "codex":
		return handleCodex(d, sid, event, body)
	case "none":
		return nil
	}
	return fmt.Errorf("unknown protocol %q", protocol)
}

type claudePayload struct {
	ToolName string `json:"tool_name"`
	Message  string `json:"message"`
	Reason   string `json:"reason"`
}

func handleClaude(d Deps, sid, event string, body []byte) error {
	var p claudePayload
	_ = json.Unmarshal(body, &p)

	var ev state.Event
	var soundEv string
	var msg string
	switch event {
	case "SessionStart":
		ev, soundEv = state.EvSessionStart, "session_start"
	case "PreToolUse":
		ev = state.EvPreToolUse
	case "PostToolUse":
		ev = state.EvPostToolUse
	case "Notification":
		ev, soundEv = state.EvNotification, "needs_input"
		msg = p.Message
	case "Stop":
		ev, soundEv = state.EvStop, "session_idle"
	case "SessionEnd":
		ev, soundEv = state.EvSessionEnd, "session_completed"
	case "SubagentStop":
		ev = state.EvPostToolUse // ish; logged but no top-level transition
	default:
		return nil
	}

	if _, err := d.State.Apply(sid, ev, msg); err != nil {
		return err
	}
	_ = d.Events(sid).Append(events.Entry{Type: event, Tool: p.ToolName, Detail: p.Message})
	if soundEv != "" && d.Config.Sound.Enabled {
		if file := d.Config.Sound.Events[soundEv]; file != "" {
			full := file
			if !filepath.IsAbs(full) {
				full = filepath.Join(d.Paths.SoundsDir(), file)
			}
			_ = d.Sound.Play(full)
		}
	}
	return nil
}

func handleCodex(d Deps, sid, event string, body []byte) error {
	// TODO: confirm exact codex hook event names against current CLI release.
	// Conceptual mapping from spec §6.2:
	//   "start" → SessionStart, "pre-tool" → PreToolUse, "post-tool" → PostToolUse,
	//   "awaiting-input" → Notification, "done" → Stop.
	return handleClaude(d, sid, mapCodexEvent(event), body)
}

func mapCodexEvent(e string) string {
	switch e {
	case "start":
		return "SessionStart"
	case "pre-tool":
		return "PreToolUse"
	case "post-tool":
		return "PostToolUse"
	case "awaiting-input":
		return "Notification"
	case "done":
		return "Stop"
	case "session-end":
		return "SessionEnd"
	}
	return e
}
