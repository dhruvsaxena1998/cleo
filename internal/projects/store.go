package projects

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/dhruvsaxena1998/cleo/internal/ids"
)

var ErrNotFound = errors.New("project not found")

type Project struct {
	ID           string    `json:"id"`
	Name         string    `json:"name"`
	Path         string    `json:"path"`
	DefaultAgent string    `json:"default_agent,omitempty"`
	AddedAt      time.Time `json:"added_at"`
}

type fileFormat struct {
	Projects []Project `json:"projects"`
}

type Store struct{ path string }

func NewStore(path string) *Store { return &Store{path: path} }

func (s *Store) Add(path string) (Project, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return Project{}, err
	}
	all, err := s.read()
	if err != nil {
		return Project{}, err
	}
	existing := map[string]bool{}
	for _, p := range all.Projects {
		existing[p.ID] = true
		if p.Path == abs {
			return Project{}, fmt.Errorf("path already registered: %s (id=%s)", abs, p.ID)
		}
	}
	id := ids.DedupeSlug(ids.Slugify(filepath.Base(abs)), existing)
	p := Project{
		ID:      id,
		Name:    filepath.Base(abs),
		Path:    abs,
		AddedAt: time.Now().UTC(),
	}
	all.Projects = append(all.Projects, p)
	return p, s.write(all)
}

func (s *Store) Remove(id string) error {
	all, err := s.read()
	if err != nil {
		return err
	}
	found := false
	out := all.Projects[:0]
	for _, p := range all.Projects {
		if p.ID == id {
			found = true
			continue
		}
		out = append(out, p)
	}
	if !found {
		return ErrNotFound
	}
	all.Projects = out
	return s.write(all)
}

func (s *Store) Get(id string) (Project, error) {
	all, err := s.read()
	if err != nil {
		return Project{}, err
	}
	for _, p := range all.Projects {
		if p.ID == id {
			return p, nil
		}
	}
	return Project{}, ErrNotFound
}

func (s *Store) List() ([]Project, error) {
	all, err := s.read()
	return all.Projects, err
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
	return f, json.Unmarshal(b, &f)
}

func (s *Store) write(f fileFormat) error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return err
	}
	tmp := s.path + ".tmp"
	b, err := json.MarshalIndent(f, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, s.path)
}
