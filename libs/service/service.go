package service

import "sync/atomic"

type State uint32

const (
	Stopped State = iota
	Running
)

// Service provides an interface to change an check service state
type Service struct {
	state uint32
}

// SetState allows to change service state
func (s *Service) SetState(state State) {
	atomic.StoreUint32(&s.state, uint32(state))
}

// State returns service current state
func (s *Service) State() State {
	return State(atomic.LoadUint32(&s.state))
}
