package reconcile

import (
	"time"

	"github.com/dhruvsaxena1998/cleo/internal/state"
)

type TmuxLs interface {
	LsPrefix(prefix string) ([]string, error)
}

func Run(st *state.Store, tx TmuxLs, idleTimeout time.Duration) error {
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
			_, _ = st.Apply(s.ID, state.EvDead, "")
			continue
		}
		if s.State == state.Idle && time.Since(s.LastEventAt) > idleTimeout {
			_, _ = st.Apply(s.ID, state.EvIdleTimeout, "")
		}
	}
	return nil
}
