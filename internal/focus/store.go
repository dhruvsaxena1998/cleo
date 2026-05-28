package focus

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// focusTTL is the maximum age of a focused=true entry before it is treated as
// stale. Self-heals crash scenarios where the Set(false) callback never ran.
const focusTTL = 5 * time.Minute

type Store struct {
	path string
}

type sessionFocus struct {
	Focused   bool      `json:"focused"`
	UpdatedAt time.Time `json:"updated_at"`
}

type fileFormat struct {
	Sessions map[string]sessionFocus `json:"sessions"`
}

func NewStore(path string) *Store { return &Store{path: path} }

func (s *Store) Set(sessionID string, focused bool) error {
	if sessionID == "" {
		return nil
	}
	f, err := s.read()
	if err != nil {
		return err
	}
	if f.Sessions == nil {
		f.Sessions = map[string]sessionFocus{}
	}
	f.Sessions[sessionID] = sessionFocus{
		Focused:   focused,
		UpdatedAt: time.Now().UTC(),
	}
	return s.write(f)
}

func (s *Store) IsFocused(sessionID string) bool {
	f, err := s.read()
	if err != nil {
		return false
	}
	sf := f.Sessions[sessionID]
	return sf.Focused && !sf.UpdatedAt.IsZero() && time.Since(sf.UpdatedAt) <= focusTTL
}

func (s *Store) read() (fileFormat, error) {
	b, err := os.ReadFile(s.path)
	if os.IsNotExist(err) {
		return fileFormat{}, nil
	}
	if err != nil {
		return fileFormat{}, err
	}
	var f fileFormat
	if err := json.Unmarshal(b, &f); err != nil {
		return fileFormat{}, err
	}
	// Prune entries older than focusTTL so focus.json stays bounded.
	// The next Set() call will persist the compacted map to disk.
	for id, sf := range f.Sessions {
		if !sf.UpdatedAt.IsZero() && time.Since(sf.UpdatedAt) > focusTTL {
			delete(f.Sessions, id)
		}
	}
	return f, nil
}

func (s *Store) write(f fileFormat) error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return err
	}
	// Per-process temp file avoids a race when multiple cleo focus invocations
	// run concurrently (e.g. client-attached and client-focus-in hooks both fire
	// on tmux attach). A shared tmp path causes the second rename to fail with
	// ENOENT after the first process has already renamed the file away.
	tmp := fmt.Sprintf("%s.tmp.%d", s.path, os.Getpid())
	defer os.Remove(tmp)
	b, err := json.MarshalIndent(f, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, s.path)
}
