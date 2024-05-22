package getters

import (
	"github.com/celestiaorg/celestia-node/pruner"
)

type Option func(*ShrexGetter)

func WithAvailabilityWindow(window pruner.AvailabilityWindow) func(*ShrexGetter) {
	return func(s *ShrexGetter) {
		s.availabilityWindow = window
	}
}
