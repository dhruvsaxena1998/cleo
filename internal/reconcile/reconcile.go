package reconcile

import (
	"time"

	"github.com/dhruvsaxena1998/cleo/internal/state"
)

type TmuxLs interface {
	LsPrefix(prefix string) ([]string, error)
}

type Options struct {
	IdleTimeout     time.Duration
	SpawningTimeout time.Duration
}

func RunOpts(st *state.Store, tx TmuxLs, opts Options) error {
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
	for _, s := range sessions {
		if !liveSet[s.ID] && s.State != state.Dead {
			_, _ = st.ApplySynthetic(s.ID, state.EvDead, "")
			continue
		}
		// If the agent has been spawning for longer than SpawningTimeout and
		// the tmux session is still alive, the hooks likely didn't fire.
		// Advance to Running so the TUI shows meaningful state.
		if s.State == state.Spawning && liveSet[s.ID] &&
			opts.SpawningTimeout > 0 && time.Since(s.StartedAt) > opts.SpawningTimeout {
			_, _ = st.Apply(s.ID, state.EvSessionStart,
				"advanced from spawning by reconciler (no startup hook seen)")
			continue
		}
		if (s.State == state.Idle || s.State == state.WaitingForInput) &&
			time.Since(s.LastEventAt) > opts.IdleTimeout {
			_, _ = st.ApplySynthetic(s.ID, state.EvIdleTimeout, "")
		}
	}
	return nil
}
