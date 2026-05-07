package cli

import (
	"testing"

	"github.com/dhruvsaxena1998/cleo/internal/state"
)

func TestPruneArchivesFinishedSessions(t *testing.T) {
	root := t.TempDir()
	c, _ := NewCtxWithRoot(root)
	for _, st := range []state.State{state.Completed, state.Errored, state.Dead, state.Running, state.Idle} {
		_ = c.State.Put(state.Session{
			ID:        "cleo-foo-claude-" + string(st),
			ProjectID: "foo", Agent: "claude", Name: string(st), State: st,
		})
	}
	cmd := newPruneCmd(func() *Ctx { return c })
	cmd.SetArgs([]string{"foo", "--keep", "0", "--yes"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	all, _ := c.State.List()
	for _, s := range all {
		if s.State.IsFinished() {
			t.Errorf("finished still present: %s", s.ID)
		}
	}
}
