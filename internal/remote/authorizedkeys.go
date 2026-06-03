// Package remote builds the SSH authorized_keys entry that lets a phone reach
// the Cleo dashboard over a forced command. The merge is pure — given the
// existing file content, a public key, and the absolute cleo path it returns
// the new content, inserting or replacing exactly one Cleo-managed line. The
// pure decision is the test surface; the file IO that consumes it is a thin
// shell (see internal/cli/remote.go).
package remote

import (
	"errors"
	"fmt"
	"strings"
)

// ManagedComment is the trailing comment that marks an authorized_keys line as
// Cleo-managed. Detection and replacement key off this marker, so a user's own
// keys (which won't carry it) are always preserved.
const ManagedComment = "cleo-remote"

// ManagedLine builds the single forced-command authorized_keys line that pins
// an incoming SSH connection to the cleo dashboard:
//
//	restrict,pty,command="<abs cleo path>" <type> <key> cleo-remote
//
// restrict is default-deny; pty re-enables the PTY the TUI needs; command=
// forces the dashboard regardless of what the client requests; the trailing
// cleo-remote comment marks the line as managed. cleoPath must be absolute —
// forced commands run without the login PATH, so a bare "cleo" would not
// resolve. The public key's own trailing comment (e.g. phone@iphone) is
// dropped so the line ends in exactly the marker and stays idempotent.
func ManagedLine(pubKey, cleoPath string) (string, error) {
	key, err := normalizePubKey(pubKey)
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(cleoPath) == "" {
		return "", errors.New("cleo path is empty")
	}
	return fmt.Sprintf("restrict,pty,command=%s %s %s", quoteCommand(cleoPath), key, ManagedComment), nil
}

// Merge returns the new authorized_keys content with exactly one Cleo-managed
// line (the trailing cleo-remote marker) inserted or replaced in place,
// preserving every other line and the surrounding content. Appending into
// empty or marker-free content adds the line; an existing managed line is
// replaced where it sits (so a changed key or path updates in place); any
// extra managed lines are collapsed to one. The result always ends in a single
// trailing newline.
func Merge(existing, pubKey, cleoPath string) (string, error) {
	line, err := ManagedLine(pubKey, cleoPath)
	if err != nil {
		return "", err
	}
	lines := splitLines(existing)
	out := make([]string, 0, len(lines)+1)
	replaced := false
	for _, l := range lines {
		if isManaged(l) {
			if !replaced {
				out = append(out, line)
				replaced = true
			}
			continue // drop any additional managed lines
		}
		out = append(out, l)
	}
	if !replaced {
		out = append(out, line)
	}
	return strings.Join(out, "\n") + "\n", nil
}

// isManaged reports whether a line is a Cleo-managed entry, detected by the
// trailing cleo-remote marker comment.
func isManaged(line string) bool {
	fields := strings.Fields(line)
	return len(fields) > 0 && fields[len(fields)-1] == ManagedComment
}

// normalizePubKey trims a public key to its "<type> <base64>" prefix, dropping
// any trailing comment and collapsing whitespace. It errors on input that is
// not at least a type and a key.
func normalizePubKey(pub string) (string, error) {
	fields := strings.Fields(pub)
	if len(fields) < 2 {
		return "", errors.New("invalid public key: expected '<type> <base64-key> [comment]'")
	}
	return fields[0] + " " + fields[1], nil
}

// quoteCommand wraps a forced-command path in double quotes, escaping the only
// two characters SSH treats as special inside command="…": backslash and quote.
func quoteCommand(path string) string {
	r := strings.NewReplacer(`\`, `\\`, `"`, `\"`)
	return `"` + r.Replace(path) + `"`
}

// splitLines splits authorized_keys content into logical lines, dropping the
// single trailing empty element a final newline produces (so a trailing
// newline is not mistaken for a blank entry). Blank lines and comments in the
// middle are preserved; "" yields no lines.
func splitLines(s string) []string {
	if s == "" {
		return nil
	}
	lines := strings.Split(s, "\n")
	if n := len(lines); n > 0 && lines[n-1] == "" {
		lines = lines[:n-1]
	}
	return lines
}
