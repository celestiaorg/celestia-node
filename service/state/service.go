package state

import logging "github.com/ipfs/go-log/v2"

var log = logging.Logger("state")

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
