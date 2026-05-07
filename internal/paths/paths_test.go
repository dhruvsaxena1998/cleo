package paths

import (
	"path/filepath"
	"testing"
)

func TestNewWithRoot(t *testing.T) {
	p := NewWithRoot("/tmp/test")
	cases := map[string]string{
		"ConfigDir":    "/tmp/test",
		"ConfigFile":   "/tmp/test/config.toml",
		"ProjectsFile": "/tmp/test/projects.json",
		"StateFile":    "/tmp/test/state.json",
		"StateLock":    "/tmp/test/state.json.lock",
		"EventsDir":    "/tmp/test/events",
		"ArchiveDir":   "/tmp/test/events/archive",
		"SoundsDir":    "/tmp/test/sounds",
		"HookErrLog":   "/tmp/test/hook-errors.log",
	}
	got := map[string]string{
		"ConfigDir":    p.ConfigDir(),
		"ConfigFile":   p.ConfigFile(),
		"ProjectsFile": p.ProjectsFile(),
		"StateFile":    p.StateFile(),
		"StateLock":    p.StateLock(),
		"EventsDir":    p.EventsDir(),
		"ArchiveDir":   p.ArchiveDir(),
		"SoundsDir":    p.SoundsDir(),
		"HookErrLog":   p.HookErrLog(),
	}
	for k, want := range cases {
		if got[k] != want {
			t.Errorf("%s: got %q want %q", k, got[k], want)
		}
	}
	// EventsLog correlates to a session id
	if got := p.EventsLog("cleo-foo-bar"); got != filepath.Join("/tmp/test/events", "cleo-foo-bar.jsonl") {
		t.Errorf("EventsLog: %q", got)
	}
}
