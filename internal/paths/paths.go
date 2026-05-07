package paths

import (
	"os"
	"path/filepath"
)

type Paths struct{ root string }

// New uses ~/.config/cleo (or $XDG_CONFIG_HOME/cleo if set).
func New() Paths {
	if x := os.Getenv("XDG_CONFIG_HOME"); x != "" {
		return Paths{root: filepath.Join(x, "cleo")}
	}
	home, _ := os.UserHomeDir()
	return Paths{root: filepath.Join(home, ".config", "cleo")}
}

func NewWithRoot(root string) Paths { return Paths{root: root} }

func (p Paths) ConfigDir() string    { return p.root }
func (p Paths) ConfigFile() string   { return filepath.Join(p.root, "config.toml") }
func (p Paths) ProjectsFile() string { return filepath.Join(p.root, "projects.json") }
func (p Paths) StateFile() string    { return filepath.Join(p.root, "state.json") }
func (p Paths) StateLock() string    { return filepath.Join(p.root, "state.json.lock") }
func (p Paths) EventsDir() string    { return filepath.Join(p.root, "events") }
func (p Paths) ArchiveDir() string   { return filepath.Join(p.root, "events", "archive") }
func (p Paths) SoundsDir() string    { return filepath.Join(p.root, "sounds") }
func (p Paths) HookErrLog() string   { return filepath.Join(p.root, "hook-errors.log") }
func (p Paths) EventsLog(sid string) string {
	return filepath.Join(p.root, "events", sid+".jsonl")
}
