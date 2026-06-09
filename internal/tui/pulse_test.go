package tui

import (
	"testing"
	"time"

	"github.com/dhruvsaxena1998/cleo/internal/projects"
	"github.com/dhruvsaxena1998/cleo/internal/state"
)

func TestPulseColorBreathesForWorkingStatesOnly(t *testing.T) {
	m := New(newTestCtx(t))
	peak, trough := 0, pulsePeriod/2

	// Running and spawning breathe: peak and trough colours differ.
	for _, s := range []string{"running", "spawning"} {
		m.animFrame = peak
		hi := m.pulseColor(s)
		m.animFrame = trough
		lo := m.pulseColor(s)
		if hi == lo {
			t.Fatalf("%s should pulse: peak and trough colours are identical (%v)", s, hi)
		}
	}

	// Every other state holds its static colour at any frame.
	for _, f := range []int{0, 3, 7, pulsePeriod / 2, pulsePeriod} {
		m.animFrame = f
		for _, s := range []string{"idle", "waiting_for_input", "completed", "error", "dead"} {
			if got := m.pulseColor(s); got != m.theme.StateColor(s) {
				t.Fatalf("%s at frame %d pulsed (%v); want static %v", s, f, got, m.theme.StateColor(s))
			}
		}
	}
}

func TestPulseTickAdvancesWhileWorkingAndStopsWhenIdle(t *testing.T) {
	m := New(newTestCtx(t))
	m.animTicking = true // pretend the loop is live

	// No working session: the tick must stop the loop and clear the guard.
	updated, cmd := m.Update(animTickMsg{})
	mm := updated.(Model)
	if cmd != nil {
		t.Fatal("idle anim tick must not re-arm the pulse")
	}
	if mm.animTicking {
		t.Fatal("anim guard should clear when no work remains")
	}

	// A working session: the tick advances the frame and re-arms.
	mm.sessions = []state.Session{{ID: "s", ProjectID: "p", Agent: "claude", State: state.Running}}
	before := mm.animFrame
	updated2, cmd2 := mm.Update(animTickMsg{})
	mm2 := updated2.(Model)
	if cmd2 == nil {
		t.Fatal("working anim tick must re-arm the pulse")
	}
	if mm2.animFrame != before+1 {
		t.Fatalf("frame = %d, want %d", mm2.animFrame, before+1)
	}
}

func TestStateLoadStartsPulseOnlyWhenWorking(t *testing.T) {
	working := stateLoadedMsg{
		projects: []projects.Project{{ID: "p"}},
		sessions: []state.Session{{ID: "s", ProjectID: "p", Agent: "claude", State: state.Running, StartedAt: time.Now()}},
	}
	updated, cmd := New(newTestCtx(t)).Update(working)
	if !updated.(Model).animTicking {
		t.Fatal("pulse should start ticking when a working session loads")
	}
	if cmd == nil {
		t.Fatal("expected a command to start the pulse loop")
	}

	idle := stateLoadedMsg{
		projects: []projects.Project{{ID: "p"}},
		sessions: []state.Session{{ID: "s", ProjectID: "p", Agent: "claude", State: state.Idle, StartedAt: time.Now()}},
	}
	u2, _ := New(newTestCtx(t)).Update(idle)
	if u2.(Model).animTicking {
		t.Fatal("pulse should stay idle when no session is working")
	}
}
