package tui

import (
	"testing"
	"time"

	"github.com/dhruvsaxena1998/cleo/internal/projects"
	"github.com/dhruvsaxena1998/cleo/internal/state"
)

func TestAnimGlyphAnimatesWorkingStatesOnly(t *testing.T) {
	m := New(newTestCtx(t)) // default nerd icons
	sp := m.theme.Icons.spinner()

	m.animFrame = 0
	if got := m.animGlyph("running"); got != sp[0] {
		t.Fatalf("running glyph = %q, want spinner frame 0 %q", got, sp[0])
	}
	m.animFrame = len(sp) + 2 // must wrap with %
	if got := m.animGlyph("running"); got != sp[2] {
		t.Fatalf("running glyph did not wrap to frame 2, got %q", got)
	}
	if got := m.animGlyph("spawning"); got != sp[2] {
		t.Fatalf("spawning should animate too, got %q", got)
	}
	// Every non-working state keeps its static marker.
	for _, s := range []string{"idle", "waiting_for_input", "completed", "error", "dead"} {
		if got := m.animGlyph(s); got != m.theme.stateGlyph(s) {
			t.Fatalf("%s glyph = %q, want static %q", s, got, m.theme.stateGlyph(s))
		}
	}
}

func TestSpinnerTickAdvancesWhileWorkingAndStopsWhenIdle(t *testing.T) {
	m := New(newTestCtx(t))
	m.animTicking = true // pretend the loop is live

	// No working session: the tick must stop the loop and clear the guard.
	updated, cmd := m.Update(animTickMsg{})
	mm := updated.(Model)
	if cmd != nil {
		t.Fatal("idle anim tick must not re-arm the spinner")
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
		t.Fatal("working anim tick must re-arm the spinner")
	}
	if mm2.animFrame != before+1 {
		t.Fatalf("frame = %d, want %d", mm2.animFrame, before+1)
	}
}

func TestStateLoadStartsSpinnerOnlyWhenWorking(t *testing.T) {
	working := stateLoadedMsg{
		projects: []projects.Project{{ID: "p"}},
		sessions: []state.Session{{ID: "s", ProjectID: "p", Agent: "claude", State: state.Running, StartedAt: time.Now()}},
	}
	updated, cmd := New(newTestCtx(t)).Update(working)
	if !updated.(Model).animTicking {
		t.Fatal("spinner should start ticking when a working session loads")
	}
	if cmd == nil {
		t.Fatal("expected a command to start the spinner loop")
	}

	idle := stateLoadedMsg{
		projects: []projects.Project{{ID: "p"}},
		sessions: []state.Session{{ID: "s", ProjectID: "p", Agent: "claude", State: state.Idle, StartedAt: time.Now()}},
	}
	u2, _ := New(newTestCtx(t)).Update(idle)
	if u2.(Model).animTicking {
		t.Fatal("spinner should stay idle when no session is working")
	}
}
