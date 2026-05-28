package reconcile

import (
	"time"

	"github.com/dhruvsaxena1998/cleo/internal/state"
)

type TmuxLs interface {
	LsPrefix(prefix string) ([]string, error)
}

type StateStore interface {
	List() ([]state.Session, error)
	Apply(id string, ev state.Event, lastMessage string) (state.Session, error)
	ApplySynthetic(id string, ev state.Event, lastMessage string) (state.Session, error)
}

type Options struct {
	IdleTimeout     time.Duration
	SpawningTimeout time.Duration
}

// Action encodes one intended state transition produced by Decide.
type Action struct {
	SessionID string
	Event     state.Event
	Message   string
	BumpTime  bool // true → use Apply (bumps LastEventAt), false → use ApplySynthetic
}

// Decide is a pure function that computes the set of state transitions
// needed for the given session snapshot. It has zero I/O and is fully
// deterministic for a given input triplet (sessions, liveSet, now).
func Decide(sessions []state.Session, liveSet map[string]bool, now time.Time, opts Options) []Action {
	var actions []Action
	for _, s := range sessions {
		// Session missing from tmux live set and not already dead → mark dead.
		if !liveSet[s.ID] && s.State != state.Dead {
			actions = append(actions, Action{
				SessionID: s.ID,
				Event:     state.EvDead,
				BumpTime:  false,
			})
			continue
		}
		// Completed session still has a live tmux — stale done record.
		// Revive to Idle and bump LastEventAt so the idle clock restarts.
		if liveSet[s.ID] && s.State == state.Completed {
			actions = append(actions, Action{
				SessionID: s.ID,
				Event:     state.EvUserResume,
				BumpTime:  true,
			})
			continue
		}
		// If the agent has been spawning for longer than SpawningTimeout and
		// the tmux session is still alive, the hooks likely didn't fire.
		if s.State == state.Spawning && liveSet[s.ID] &&
			opts.SpawningTimeout > 0 && now.Sub(s.StartedAt) > opts.SpawningTimeout {
			actions = append(actions, Action{
				SessionID: s.ID,
				Event:     state.EvSessionStart,
				Message:   "advanced from spawning by reconciler (no startup hook seen)",
				BumpTime:  true,
			})
			continue
		}
		// Idle or WaitingForInput past the idle timeout → synthetic timeout.
		if (s.State == state.Idle || s.State == state.WaitingForInput) &&
			now.Sub(s.LastEventAt) > opts.IdleTimeout {
			actions = append(actions, Action{
				SessionID: s.ID,
				Event:     state.EvIdleTimeout,
				BumpTime:  false,
			})
		}
	}
	return actions
}

// ApplyActions applies a set of actions to a StateStore.
func ApplyActions(st StateStore, actions []Action) error {
	for _, a := range actions {
		var err error
		if a.BumpTime {
			_, err = st.Apply(a.SessionID, a.Event, a.Message)
		} else {
			_, err = st.ApplySynthetic(a.SessionID, a.Event, a.Message)
		}
		if err != nil {
			return err
		}
	}
	return nil
}

// RunOpts reconciles the state store against live tmux sessions.
// It is a thin wrapper: gather data, call Decide, call ApplyActions.
func RunOpts(st StateStore, tx TmuxLs, opts Options) error {
	live, err := tx.LsPrefix("cleo-")
	if err != nil {
		return err
	}
	liveSet := map[string]bool{}
	for _, n := range live {
		liveSet[n] = true
	}
	sessions, err := st.List()
	if err != nil {
		return err
	}
	actions := Decide(sessions, liveSet, time.Now(), opts)
	return ApplyActions(st, actions)
}
