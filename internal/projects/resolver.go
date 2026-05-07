package projects

import (
	"path/filepath"
	"strings"
)

func (s *Store) ResolveFromCwd(cwd string) (Project, error) {
	abs, err := filepath.Abs(cwd)
	if err != nil {
		return Project{}, err
	}
	all, err := s.List()
	if err != nil {
		return Project{}, err
	}
	// Match longest prefix wins (in case of nested registered paths).
	var best Project
	bestLen := -1
	for _, p := range all {
		if abs == p.Path || strings.HasPrefix(abs, p.Path+string(filepath.Separator)) {
			if len(p.Path) > bestLen {
				bestLen = len(p.Path)
				best = p
			}
		}
	}
	if bestLen < 0 {
		return Project{}, ErrNotFound
	}
	return best, nil
}
