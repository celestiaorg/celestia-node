package state

// Service // TODO document
type Service struct {
	accessor Accessor
}

// NewService constructs a new state Service.
func NewService(accessor Accessor) *Service {
	return &Service{
		accessor: accessor,
	}
}
