package templates

func stateTimelineStates() []string {
	return []string{
		"PENDING_FREELANCER",
		"PENDING_DEPOSIT",
		"DEPOSIT_PENDING_CONFIRM",
		"FUNDED",
		"DELIVERED",
		"COMPLETED",
	}
}

func isPastState(state, currentState string) bool {
	if state == currentState {
		return false
	}
	for _, s := range stateTimelineStates() {
		if s == currentState {
			return true
		}
		if s == state {
			return false
		}
	}
	return false
}
