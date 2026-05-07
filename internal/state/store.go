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

// Apply transitions a session by event under the lock and returns the updated session.
// `lastMessage` is set on the session if non-empty (used for Notification text).
func (s *Store) Apply(id string, ev Event, lastMessage string) (Session, error) {
	var out Session
	err := s.modify(func(f *fileFormat) error {
		sess, ok := f.Sessions[id]
		if !ok {
			return ErrSessionNotFound
		}
		sess.State = NextState(sess.State, ev)
		sess.LastEventAt = time.Now().UTC()
		if lastMessage != "" {
			sess.LastMessage = lastMessage
		}
		if ev == EvPostToolUse {
			sess.ToolCount++
		}
		f.Sessions[id] = sess
		out = sess
		return nil
	})
	return out, err
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
