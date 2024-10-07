package share

import (
	"context"
	"errors"

	"github.com/celestiaorg/celestia-node/header"
)

// ErrNotAvailable is returned whenever DA sampling fails.
var ErrNotAvailable = errors.New("share: data not available")

// Availability defines interface for validation of Shares' availability.
//
//go:generate mockgen -destination=availability/mocks/availability.go -package=mocks . Availability
type Availability interface {
	// SharesAvailable subjectively validates if Shares committed to the given Root are available on
	// the Network.
	SharesAvailable(context.Context, *header.ExtendedHeader) error
}
