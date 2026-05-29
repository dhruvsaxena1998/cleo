package cli

import (
	"errors"
	"fmt"
	"strings"

	"github.com/dhruvsaxena1998/cleo/internal/hooks"
)

// parseAgentsFlag normalizes and validates a comma-separated list of
// agent names supplied via --agents. Accepted names come from the
// hooks.Protocols() registry, so adding an agent needs no edit here.
// See agents_test.go for the rules.
func parseAgentsFlag(raw string) ([]string, error) {
	if raw == "" {
		return nil, errors.New("--agents requires at least one agent name")
	}
	known := make(map[string]struct{})
	for _, n := range hooks.ProtocolNames() {
		known[n] = struct{}{}
	}
	parts := strings.Split(raw, ",")
	seen := make(map[string]struct{}, len(parts))
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		name := strings.ToLower(p)
		if _, ok := known[name]; !ok {
			return nil, fmt.Errorf("unknown agent %q (accepted: %s)", name, strings.Join(hooks.ProtocolNames(), ", "))
		}
		if _, dup := seen[name]; dup {
			continue
		}
		seen[name] = struct{}{}
		out = append(out, name)
	}
	return out, nil
}
