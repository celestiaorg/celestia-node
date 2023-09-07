package share

import (
	"context"
	"errors"

	"github.com/celestiaorg/celestia-app/pkg/da"
)

// ErrNotAvailable is returned whenever DA sampling fails.
var ErrNotAvailable = errors.New("share: data not available")

// Root represents root commitment to multiple Shares.
// In practice, it is a commitment to all the Data in a square.
type Root = da.DataAvailabilityHeader

// Availability defines interface for validation of Shares' availability.
type Availability interface {
	// SharesAvailable subjectively validates if Shares committed to the given Root are available on
	// the Network.
	SharesAvailable(context.Context, *Root) error
	// ProbabilityOfAvailability calculates the probability of the data square
	// being available based on the number of samples collected.
	// TODO(@Wondertan): Merge with SharesAvailable method, eventually
	ProbabilityOfAvailability(context.Context) float64
}
