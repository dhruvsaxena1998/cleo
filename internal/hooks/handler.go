package hooks

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

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
	// Focused returns true when the target tmux session is currently focused.
	// It is best-effort; nil means sound should not be suppressed.
	Focused func(sid string) bool
	// Now returns the cleo session id from env or returns an error if absent.
	// Test seam.
	Now func() (string, error)
	// FindByCwd is an optional fallback for when CLEO_SESSION_ID is absent from
	// the hook subprocess env (codex may not propagate parent env to hooks).
	// Given the working directory and agent name, it returns the active session ID.
	FindByCwd func(cwd, agent string) (string, error)
}

func DefaultNow() (string, error) {
	sid := os.Getenv("CLEO_SESSION_ID")
	if sid == "" {
		return "", errNoSession
	}
	return sid, nil
}

var errNoSession = fmt.Errorf("CLEO_SESSION_ID not set")

// baseHookPayload contains common fields hook providers send on events.
type baseHookPayload struct {
	Cwd string `json:"cwd"`
}

func Handle(d Deps, protocol, event string, stdin io.Reader, stdout io.Writer) error {
	// Read stdin first — the cwd fallback needs it before we return early.
	body, _ := io.ReadAll(stdin)

	trace := hookTrace{Protocol: protocol, Event: event, EnvSession: os.Getenv("CLEO_SESSION_ID") != ""}
	sid, err := d.Now()
	if err == nil {
		trace.ResolvedSession = sid
	}
	// FindByCwd is only used for codex: Claude propagates env vars to hook
	// subprocesses, so absent CLEO_SESSION_ID for Claude unambiguously means a
	// standalone session that should not be attributed to any cleo session.
	// Codex may strip the parent env from hook subprocesses, so the cwd-based
	// lookup is needed there as a best-effort fallback.
	if err != nil && d.FindByCwd != nil && protocol == "codex" {
		// CLEO_SESSION_ID may not be present in Codex hook subprocess environments.
		// Use hook payload cwd when available; otherwise fall back to the hook
		// process working directory.
		var base baseHookPayload
		_ = json.Unmarshal(body, &base)
		trace.Cwd = base.Cwd
		if base.Cwd == "" {
			if wd, wdErr := os.Getwd(); wdErr == nil {
				base.Cwd = wd
				trace.Cwd = wd
			}
		}
		if base.Cwd != "" {
			sid, err = d.FindByCwd(base.Cwd, protocol)
			trace.ResolvedSession = sid
		}
	}
	if err != nil || sid == "" {
		trace.Result = "ignored:no_session"
		logHookTrace(d.Paths, trace)
		return nil // no session to attribute this event to
	}
	trace.Result = "resolved"
	logHookTrace(d.Paths, trace)

	var herr error
	switch protocol {
	case "claude":
		herr = handleClaude(d, sid, event, body)
	case "codex":
		herr = handleCodex(d, sid, event, body)
	case "none":
		return nil
	default:
		herr = fmt.Errorf("unknown protocol %q", protocol)
	}
	if herr != nil {
		logHookErr(d.Paths, protocol, event, herr)
	}
	return herr
}

type hookTrace struct {
	Protocol        string `json:"protocol"`
	Event           string `json:"event"`
	EnvSession      bool   `json:"env_session"`
	Cwd             string `json:"cwd,omitempty"`
	ResolvedSession string `json:"resolved_session,omitempty"`
	Result          string `json:"result"`
}

func logHookTrace(p paths.Paths, trace hookTrace) {
	f, e := os.OpenFile(p.HookTraceLog(), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if e != nil {
		return
	}
	defer f.Close()
	row := struct {
		At string `json:"at"`
		hookTrace
	}{
		At:        time.Now().Format(time.RFC3339),
		hookTrace: trace,
	}
	b, _ := json.Marshal(row)
	fmt.Fprintln(f, string(b))
}

func logHookErr(p paths.Paths, protocol, event string, err error) {
	f, e := os.OpenFile(p.HookErrLog(), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if e != nil {
		return
	}
	defer f.Close()
	fmt.Fprintf(f, "%s [%s/%s] %v\n", time.Now().Format(time.RFC3339), protocol, event, err)
}

type claudePayload struct {
	ToolName string `json:"tool_name"`
	Message  string `json:"message"`
	Reason   string `json:"reason"`
}

func handleClaude(d Deps, sid, event string, body []byte) error {
	var p claudePayload
	_ = json.Unmarshal(body, &p)

	// SubagentStop is only logged — it must not affect the main session state
	// because it fires after every sub-conversation and would spuriously bounce
	// Idle → Running on every subagent completion.
	if event == "SubagentStop" {
		_ = d.Events(sid).Append(events.Entry{Type: event, Tool: p.ToolName})
		return nil
	}

	var ev state.Event
	var soundEv string
	var msg string
	switch event {
	case "SessionStart":
		ev, soundEv = state.EvSessionStart, "session_start"
	case "UserPromptSubmit":
		ev = state.EvUserResume
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
	default:
		return nil
	}

	if _, err := d.State.Apply(sid, ev, msg); err != nil {
		return err
	}
	_ = d.Events(sid).Append(events.Entry{Type: event, Tool: p.ToolName, Detail: p.Message})
	if soundEv != "" && d.Config.SoundEventEnabled(soundEv) && !sessionFocused(d, sid) {
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

func sessionFocused(d Deps, sid string) bool {
	return d.Focused != nil && d.Focused(sid)
}

func handleCodex(d Deps, sid, event string, body []byte) error {
	// Codex uses the same PascalCase event names as Claude Code, with one
	// difference: PermissionRequest (approval needed) maps to Notification
	// (waiting for input). Synthesize a Claude-compatible payload for it.
	if event == "PermissionRequest" {
		var pr struct {
			ToolName  string `json:"tool_name"`
			ToolInput struct {
				Command     string `json:"command"`
				Description string `json:"description"`
			} `json:"tool_input"`
		}
		_ = json.Unmarshal(body, &pr)
		msg := pr.ToolName
		if pr.ToolInput.Command != "" {
			msg = pr.ToolInput.Command
		} else if pr.ToolInput.Description != "" {
			msg = pr.ToolInput.Description
		}
		synthetic, _ := json.Marshal(claudePayload{ToolName: pr.ToolName, Message: msg})
		return handleClaude(d, sid, "Notification", synthetic)
	}
	// SessionStart, UserPromptSubmit, PreToolUse, PostToolUse, Stop —
	// identical to Claude-style lifecycle events.
	return handleClaude(d, sid, event, body)
}
