package state

import "testing"

func TestNextState(t *testing.T) {
	cases := []struct {
		from State
		ev   Event
		want State
	}{
		{Spawning, EvSessionStart, Running},
		{Spawning, EvPreToolUse, Running},
		{Running, EvPreToolUse, Running}, // no-op
		{Running, EvPostToolUse, Running},
		{Running, EvNotification, WaitingForInput},
		{WaitingForInput, EvUserResume, Running},
		{Running, EvStop, Idle},
		{Idle, EvSessionEnd, Completed},
		{Idle, EvIdleTimeout, Completed},
		{Running, EvSessionEnd, Completed},
		{Idle, EvError, Errored},
	}
	for _, c := range cases {
		got := NextState(c.from, c.ev)
		if got != c.want {
			t.Errorf("NextState(%s, %s) = %s, want %s", c.from, c.ev, got, c.want)
		}
	}
}
