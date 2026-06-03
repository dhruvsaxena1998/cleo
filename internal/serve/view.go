package serve

import (
	"sort"
	"time"

	"github.com/dhruvsaxena1998/cleo/internal/state"
)

// statePrio drives the urgency sort: the session that needs you sorts first.
// This ordering is owned server-side; the page only renders.
var statePrio = map[state.State]int{
	state.WaitingForInput: 0,
	state.Errored:         1,
	state.Running:         2,
	state.Spawning:        3,
	state.Idle:            4,
	state.Completed:       5,
	state.Dead:            6,
}

func prio(s state.State) int {
	if p, ok := statePrio[s]; ok {
		return p
	}
	return len(statePrio) // unknown states sort last
}

// loud reports whether a state needs your attention. Only these two carry a
// text label on the page ("calm is quiet, attention is loud").
func loud(s state.State) bool {
	return s == state.WaitingForInput || s == state.Errored
}

// ViewSession is the deliberately narrow projection of a Session that the
// remote view exposes: agent, name, state, and age only. It carries no
// Session ID and no LastMessage — that is the security boundary from
// ADR 0004 (a pane preview or last message can leak secrets), enforced
// here by the shape of the struct.
type ViewSession struct {
	Agent      string `json:"agent"`
	Name       string `json:"name"`
	State      string `json:"state"`
	AgeSeconds int    `json:"age_seconds"`
	Attn       bool   `json:"attn"`
}

// ViewProject groups the sessions of one project for the grouped-by-project
// layout (Variant C).
type ViewProject struct {
	Project        string        `json:"project"`
	NeedsAttention bool          `json:"needs_attention"`
	Sessions       []ViewSession `json:"sessions"`
}

// View is the whole payload the remote page renders.
type View struct {
	NeedCount int           `json:"need_count"`
	Projects  []ViewProject `json:"projects"`
}

// Project builds the remote view from the live session list. It is pure —
// age is computed relative to the passed-in now — so it is fully testable
// without a clock.
func Project(sessions []state.Session, now time.Time) View {
	groups := map[string][]ViewSession{}
	var order []string
	needCount := 0
	for _, s := range sessions {
		if _, seen := groups[s.ProjectID]; !seen {
			order = append(order, s.ProjectID)
		}
		attn := loud(s.State)
		if attn {
			needCount++
		}
		groups[s.ProjectID] = append(groups[s.ProjectID], ViewSession{
			Agent:      s.Agent,
			Name:       s.Name,
			State:      string(s.State),
			AgeSeconds: ageSeconds(s, now),
			Attn:       attn,
		})
	}

	v := View{NeedCount: needCount}
	for _, pid := range order {
		items := groups[pid]
		sort.SliceStable(items, func(i, j int) bool {
			pi, pj := prio(state.State(items[i].State)), prio(state.State(items[j].State))
			if pi != pj {
				return pi < pj
			}
			return items[i].AgeSeconds < items[j].AgeSeconds // newer first
		})
		needsAttn := false
		for _, s := range items {
			if s.Attn {
				needsAttn = true
				break
			}
		}
		v.Projects = append(v.Projects, ViewProject{
			Project:        pid,
			NeedsAttention: needsAttn,
			Sessions:       items,
		})
	}

	// Order projects by their most-urgent (already-sorted first) session, so a
	// project that newly needs you floats to the top.
	sort.SliceStable(v.Projects, func(i, j int) bool {
		pi := prio(state.State(v.Projects[i].Sessions[0].State))
		pj := prio(state.State(v.Projects[j].Sessions[0].State))
		if pi != pj {
			return pi < pj
		}
		return v.Projects[i].Sessions[0].AgeSeconds < v.Projects[j].Sessions[0].AgeSeconds
	})
	return v
}

// ageSeconds is the time since the session was last active, in whole seconds.
// It prefers LastEventAt and falls back to StartedAt, mirroring `cleo ls`.
func ageSeconds(s state.Session, now time.Time) int {
	t := s.LastEventAt
	if t.IsZero() {
		t = s.StartedAt
	}
	if t.IsZero() {
		return 0
	}
	d := now.Sub(t)
	if d < 0 {
		return 0
	}
	return int(d.Seconds())
}
