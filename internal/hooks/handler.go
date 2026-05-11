package hooks

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/dhruvsaxena1998/cleo/internal/config"
	"github.com/dhruvsaxena1998/cleo/internal/events"
	"github.com/dhruvsaxena1998/cleo/internal/paths"
	"github.com/dhruvsaxena1998/cleo/internal/state"
)

// StateStore is the subset of *state.Store used by the hook handler.
type StateStore interface {
	Apply(id string, ev state.Event, msg string) (state.Session, error)
	Get(id string) (state.Session, error)
}

type Player interface {
	Play(file string) error
	Available() bool
}

type Deps struct {
	Paths  paths.Paths
	State  StateStore
	Config config.Config
	Events func(sid string) *events.Log
	Sound  Player
	Focused func(sid string) bool
	Now     func() (string, error)
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

// Handle dispatches a hook event from any supported protocol.
// payload is the raw JSON body — from stdin (Claude/Codex) or --payload flag (Pi).
func Handle(d Deps, protocol, event string, payload []byte) error {
	proto, ok := findProtocol(Protocols(), protocol)
	if !ok {
		err := errUnknownProtocol(protocol)
		logHookErr(d.Paths, protocol, event, err)
		return err
	}

	sid := resolveSession(d, proto, event, payload)
	if sid == "" {
		return nil
	}

	norm, ok := proto.Normalize(event, payload)
	if !ok {
		return nil
	}

	return applyNormalized(d, sid, norm)
}

// resolveSession finds the cleo session ID for an incoming hook.
// Strategy A: CLEO_SESSION_ID env var (all protocols).
// Strategy B: cwd lookup (only when proto.UsesCwdFallback() is true).
func resolveSession(d Deps, proto Protocol, event string, payload []byte) string {
	trace := hookTrace{Protocol: proto.Name(), Event: event, EnvSession: os.Getenv("CLEO_SESSION_ID") != ""}

	sid, err := d.Now()
	if err == nil {
		if d.State != nil {
			if _, sErr := d.State.Get(sid); sErr != nil {
				trace.FallbackReason = "env_unknown_session"
				err = sErr
				sid = ""
			} else {
				trace.FallbackReason = "env_present"
				trace.ResolvedSession = sid
			}
		} else {
			trace.FallbackReason = "env_present"
			trace.ResolvedSession = sid
		}
	} else {
		trace.FallbackReason = "env_missing"
	}

	staleSid := trace.FallbackReason == "env_unknown_session"
	if (err != nil || sid == "") && d.FindByCwd != nil && (proto.UsesCwdFallback() || staleSid) {
		var base struct {
			Cwd string `json:"cwd"`
		}
		_ = json.Unmarshal(payload, &base)
		trace.Cwd = base.Cwd
		if base.Cwd == "" {
			if wd, wdErr := os.Getwd(); wdErr == nil {
				base.Cwd = wd
				trace.Cwd = wd
			}
		}
		if base.Cwd != "" {
			resolved, fbErr := d.FindByCwd(base.Cwd, proto.Name())
			if fbErr != nil || resolved == "" {
				trace.FallbackReason = "no_match"
				err = fbErr
			} else {
				trace.ResolvedSession = resolved
				sid = resolved
				err = nil
			}
		}
	}

	if err != nil || sid == "" {
		trace.Result = "ignored:no_session"
		logHookTrace(d.Paths, trace)
		if trace.FallbackReason == "no_match" {
			logHookErr(d.Paths, proto.Name(), event, fmt.Errorf("no session matched cwd=%q", trace.Cwd))
		}
		return ""
	}
	trace.Result = "resolved"
	logHookTrace(d.Paths, trace)
	return sid
}

// applyNormalized applies a NormalizedEvent to state, event log, and sound.
func applyNormalized(d Deps, sid string, norm NormalizedEvent) error {
	// Read the pre-transition state so idle-nudge detection can check the
	// "from" state after Apply has already mutated it.
	var fromState state.State
	if d.State != nil {
		if sess, err := d.State.Get(sid); err == nil {
			fromState = sess.State
		}
	}

	var applyErr error
	if !norm.LogOnly && d.State != nil {
		if _, err := d.State.Apply(sid, norm.StateEvent, norm.Message); err != nil {
			applyErr = err
			// continue — still log event and play sound; the agent notified us
		}
	}
	entryType := string(norm.StateEvent)
	if norm.LogType != "" {
		entryType = norm.LogType
	}
	_ = d.Events(sid).Append(events.Entry{
		Type:   entryType,
		Tool:   norm.ToolName,
		Detail: norm.Message,
	})

	// Idle-nudge suppression: a Notification that arrives while the session is
	// already Idle (set by the preceding Stop) is Claude's ~60s internal timer,
	// not a genuine blocking request. Suppress the sound; the state transition to
	// WaitingForInput still happens so the TUI shows the visual indicator.
	idleNudge := norm.SuppressWhenIdle && fromState == state.Idle

	if norm.SoundEvent != "" && d.Config.SoundEventEnabled(norm.SoundEvent) && !sessionFocused(d, sid) && !idleNudge {
		playSound(d, norm.SoundEvent)
	}
	return applyErr
}

func sessionFocused(d Deps, sid string) bool {
	return d.Focused != nil && d.Focused(sid)
}

func playSound(d Deps, soundEvent string) {
	if !d.Sound.Available() {
		return
	}
	file := d.Config.Sound.Events[soundEvent]
	if file == "" {
		return
	}
	full := file
	if !filepath.IsAbs(full) {
		full = filepath.Join(d.Paths.SoundsDir(), file)
	}
	_ = d.Sound.Play(full)
}

type hookTrace struct {
	Protocol        string `json:"protocol"`
	Event           string `json:"event"`
	EnvSession      bool   `json:"env_session"`
	Cwd             string `json:"cwd,omitempty"`
	ResolvedSession string `json:"resolved_session,omitempty"`
	Result          string `json:"result"`
	FallbackReason  string `json:"fallback_reason,omitempty"`
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
