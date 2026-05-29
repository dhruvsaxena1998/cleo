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
	Paths     paths.Paths
	State     StateStore
	Config    config.Config
	Events    func(sid string) *events.Log
	Sound     Player
	Focused   func(sid string) bool
	Now       func() (string, error)
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

// applyNormalized turns a NormalizedEvent into its Hook outcome and performs
// it: gather the inputs the decision needs (IO), decide (pure), then apply the
// transition, event-log entry, and sound (IO).
//
// The idle-nudge rule reads the pre-transition state: a Notification arriving
// while the session is already Idle (set by the preceding Stop) is Claude's
// ~60s internal timer, not a genuine blocking request, so its sound is
// suppressed — the transition still happens so the TUI shows the indicator.
// decideHook owns that rule; here we just hand it the "from" state.
func applyNormalized(d Deps, sid string, norm NormalizedEvent) error {
	var fromState state.State
	if d.State != nil {
		if sess, err := d.State.Get(sid); err == nil {
			fromState = sess.State
		}
	}
	// Guard the lookups on SoundEvent so the no-sound path stays read-free.
	soundEnabled := norm.SoundEvent != "" && d.Config.SoundEventEnabled(norm.SoundEvent)
	focused := norm.SoundEvent != "" && sessionFocused(d, sid)

	out := decideHook(norm, fromState, soundEnabled, focused)

	var applyErr error
	if out.Transition != "" && d.State != nil {
		if _, err := d.State.Apply(sid, out.Transition, out.Message); err != nil {
			applyErr = err
			// continue — still log event and play sound; the agent notified us
		}
	}
	_ = d.Events(sid).Append(out.LogEntry)
	if out.SoundEvent != "" {
		if out.PlaySound {
			playSound(d, out.SoundEvent)
		}
		logSoundDecision(d.Paths, soundDecision{
			SessionID:  sid,
			SoundEvent: out.SoundEvent,
			Reason:     string(out.SoundReason),
		})
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
	event, ok := d.Config.Sound.Events[soundEvent]
	if !ok || event.File == "" {
		return
	}
	full := event.File
	if !filepath.IsAbs(full) {
		full = filepath.Join(d.Paths.SoundsDir(), event.File)
	}
	_ = d.Sound.Play(full)
}

type soundDecision struct {
	SessionID  string `json:"session_id"`
	SoundEvent string `json:"sound_event"`
	Reason     string `json:"reason"` // played | focus | idle-nudge | disabled
}

func logSoundDecision(p paths.Paths, d soundDecision) {
	f, err := os.OpenFile(p.HookTraceLog(), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	defer f.Close()
	row := struct {
		At string `json:"at"`
		soundDecision
	}{
		At:            time.Now().Format(time.RFC3339),
		soundDecision: d,
	}
	b, _ := json.Marshal(row)
	fmt.Fprintln(f, string(b))
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
