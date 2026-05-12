package state

func NextState(from State, ev Event) State {
	// Dead and Errored are hard terminal: only EvDead escapes them.
	// Completed can be revived by EvUserResume (user re-attach or reconciler
	// detecting a live tmux session on a stale done record).
	if from == Dead || from == Errored {
		if ev != EvDead {
			return from
		}
	}
	if from == Completed && ev != EvDead && ev != EvUserResume {
		return from
	}
	switch ev {
	case EvDead:
		return Dead
	case EvError:
		return Errored
	case EvSessionEnd:
		return Completed
	case EvIdleTimeout:
		if from == WaitingForInput {
			// Downgrade to idle first; another timeout cycle will complete it.
			return Idle
		}
		if from == Idle {
			return Completed
		}
		return from
	case EvSessionStart:
		return Running
	case EvNotification:
		return WaitingForInput
	case EvStop:
		return Idle
	case EvUserResume:
		// Reviving a Completed session: go to Idle so the idle clock restarts
		// and subsequent hook events can update the state normally.
		if from == Completed {
			return Idle
		}
		return Running
	case EvPreToolUse, EvPostToolUse:
		if from == Spawning {
			return Running
		}
		if from == WaitingForInput {
			return Running // implicit resume
		}
		if from == Idle {
			return Running
		}
		return from
	}
	return from
}
