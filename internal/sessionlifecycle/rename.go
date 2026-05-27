package sessionlifecycle

import (
	"errors"
	"fmt"

	"github.com/dhruvsaxena1998/cleo/internal/ids"
	"github.com/dhruvsaxena1998/cleo/internal/state"
)

// RenameResult describes the outcome of renaming a Session.
type RenameResult struct {
	SessionID string
	OldName   string
	NewName   string
}

// Rename updates the display name of a Session (the tmux session ID is unchanged).
// The new name is slugified before storage. Returns ErrSessionNotFound when the
// Session does not exist.
func (l *Lifecycle) Rename(sessionID, newName string) (RenameResult, error) {
	sess, err := l.state.Get(sessionID)
	if err != nil {
		if errors.Is(err, state.ErrSessionNotFound) {
			return RenameResult{}, fmt.Errorf("%w: %s", ErrSessionNotFound, sessionID)
		}
		return RenameResult{}, err
	}

	oldName := sess.Name
	sess.Name = ids.Slugify(newName)

	if err := l.state.Put(sess); err != nil {
		return RenameResult{}, err
	}

	return RenameResult{
		SessionID: sessionID,
		OldName:   oldName,
		NewName:   sess.Name,
	}, nil
}
