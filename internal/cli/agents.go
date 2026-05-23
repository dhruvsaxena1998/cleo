package cli

import (
	"errors"
	"fmt"
	"strings"
)

// knownAgents enumerates the agent names accepted by --agents.
// Aligned with internal/hooks.Protocols() registration order.
var knownAgents = map[string]struct{}{
	hookClaude:   {},
	hookCodex:    {},
	hookOpenCode: {},
	hookPi:       {},
}

// parseAgentsFlag normalizes and validates a comma-separated list of
// agent names supplied via --agents. See agents_test.go for the rules.
func parseAgentsFlag(raw string) ([]string, error) {
	if raw == "" {
		return nil, errors.New("--agents requires at least one agent name")
	}
	parts := strings.Split(raw, ",")
	seen := make(map[string]struct{}, len(parts))
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		name := strings.ToLower(p)
		if _, ok := knownAgents[name]; !ok {
			return nil, fmt.Errorf("unknown agent %q (accepted: claude, codex, opencode, pi)", name)
		}
		if _, dup := seen[name]; dup {
			continue
		}
		seen[name] = struct{}{}
		out = append(out, name)
	}
	return out, nil
}
