package utils

import "sync/atomic"

type State uint32

const (
	Stopped State = iota
	Running
)

// StateWrapper provides an interface to change an check service state
type StateWrapper struct {
	state uint32
}

func NewStateWrapper() *StateWrapper {
	return &StateWrapper{}
}

// SetState allows to change service state
func (s *StateWrapper) SetState(state State) {
	atomic.StoreUint32(&s.state, uint32(state))
}

// State returns service current state
func (s *StateWrapper) State() State {
	return State(atomic.LoadUint32(&s.state))
}
