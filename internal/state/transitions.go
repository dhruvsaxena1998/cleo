package state

func NextState(from State, ev Event) State {
	switch ev {
	case EvDead:
		return Dead
	case EvError:
		return Errored
	case EvSessionEnd:
		return Completed
	case EvIdleTimeout:
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
