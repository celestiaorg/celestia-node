package state

// Service can access state-related information via the given
// Accessor.
type Service struct {
	accessor Accessor
}

// NewService constructs a new state Service.
func NewService(accessor Accessor) *Service {
	return &Service{
		accessor: accessor,
	}
}
