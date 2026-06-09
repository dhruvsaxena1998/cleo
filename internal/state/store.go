package state

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"time"

	"github.com/gofrs/flock"
)

var ErrSessionNotFound = errors.New("session not found")

type fileFormat struct {
	Version  int                `json:"version"`
	Sessions map[string]Session `json:"sessions"`
}

type Store struct {
	path     string
	lockPath string
}

func NewStore(path, lockPath string) *Store {
	return &Store{path: path, lockPath: lockPath}
}

// Put inserts or overwrites a session record wholesale under the lock. Use it
// for inserting a brand-new session; to mutate an existing record use Update
// (or Apply/ApplySynthetic for state transitions) so concurrent writers are not
// clobbered by a stale read-modify-write.
func (s *Store) Put(sess Session) error {
	return s.modify(func(f *fileFormat) error {
		if f.Sessions == nil {
			f.Sessions = map[string]Session{}
		}
		f.Sessions[sess.ID] = sess
		return nil
	})
}

func (s *Store) Get(id string) (Session, error) {
	f, err := s.read()
	if err != nil {
		return Session{}, err
	}
	sess, ok := f.Sessions[id]
	if !ok {
		return Session{}, ErrSessionNotFound
	}
	return sess, nil
}

func (s *Store) List() ([]Session, error) {
	f, err := s.read()
	if err != nil {
		return nil, err
	}
	out := make([]Session, 0, len(f.Sessions))
	for _, v := range f.Sessions {
		out = append(out, v)
	}
	return out, nil
}

func (s *Store) Delete(id string) error {
	return s.modify(func(f *fileFormat) error {
		delete(f.Sessions, id)
		return nil
	})
}

// Update atomically applies mutate to the named session under the write lock
// and returns the updated record. The entire read-modify-write happens inside
// the lock, so a concurrent writer cannot clobber the result. mutate edits the
// session in place; returning an error from mutate aborts the write and
// surfaces that error to the caller. Returns ErrSessionNotFound if the session
// does not exist. State transitions must go through Apply/ApplySynthetic so the
// transition table stays the single authority over State; Update is for other
// fields (e.g. Name).
func (s *Store) Update(id string, mutate func(*Session) error) (Session, error) {
	var out Session
	err := s.modify(func(f *fileFormat) error {
		sess, ok := f.Sessions[id]
		if !ok {
			return ErrSessionNotFound
		}
		if err := mutate(&sess); err != nil {
			return err
		}
		f.Sessions[id] = sess
		out = sess
		return nil
	})
	return out, err
}

// Apply transitions a session by event under the lock and returns the updated session.
// `lastMessage` is set on the session if non-empty (used for Notification text).
func (s *Store) Apply(id string, ev Event, lastMessage string) (Session, error) {
	return s.Update(id, func(sess *Session) error {
		sess.State = NextState(sess.State, ev)
		sess.LastEventAt = time.Now().UTC()
		if lastMessage != "" {
			sess.LastMessage = lastMessage
		}
		if ev == EvPostToolUse {
			sess.ToolCount++
		}
		return nil
	})
}

// ApplySynthetic transitions a session by a reconciler-driven event without
// bumping LastEventAt. Use this for synthetic events (EvIdleTimeout, EvDead)
// that represent the absence of activity rather than activity itself.
// Bumping LastEventAt for these would reset idle timers and prevent stuck
// sessions from progressing.
func (s *Store) ApplySynthetic(id string, ev Event, lastMessage string) (Session, error) {
	return s.Update(id, func(sess *Session) error {
		sess.State = NextState(sess.State, ev)
		if lastMessage != "" {
			sess.LastMessage = lastMessage
		}
		return nil
	})
}

func (s *Store) modify(fn func(*fileFormat) error) error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return err
	}
	lk := flock.New(s.lockPath)
	if err := lk.Lock(); err != nil {
		return err
	}
	defer lk.Unlock()

	f, err := s.readUnlocked()
	if err != nil {
		return err
	}
	if err := fn(&f); err != nil {
		return err
	}
	return s.writeUnlocked(f)
}

func (s *Store) read() (fileFormat, error) {
	lk := flock.New(s.lockPath)
	if err := lk.RLock(); err != nil {
		return fileFormat{}, err
	}
	defer lk.Unlock()
	return s.readUnlocked()
}

func (s *Store) readUnlocked() (fileFormat, error) {
	b, err := os.ReadFile(s.path)
	if os.IsNotExist(err) {
		return fileFormat{Version: 1, Sessions: map[string]Session{}}, nil
	}
	if err != nil {
		return fileFormat{}, err
	}
	var f fileFormat
	if err := json.Unmarshal(b, &f); err != nil {
		return fileFormat{}, err
	}
	if f.Sessions == nil {
		f.Sessions = map[string]Session{}
	}
	if f.Version == 0 {
		f.Version = 1
	}
	return f, nil
}

func (s *Store) writeUnlocked(f fileFormat) error {
	b, err := json.MarshalIndent(f, "", "  ")
	if err != nil {
		return err
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, s.path)
}
