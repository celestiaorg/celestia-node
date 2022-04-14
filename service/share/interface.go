package share

import (
	"context"
	"errors"
	"time"
)

// ErrNotAvailable is returned whenever DA sampling fails.
var ErrNotAvailable = errors.New("da: data not available")

// AvailabilityTimeout specifies timeout for DA validation during which data have to be found on the network,
// otherwise ErrNotAvailable is fired.
// TODO: https://github.com/celestiaorg/celestia-node/issues/10
const AvailabilityTimeout = 20 * time.Minute

// Availability defines interface for validation of Shares' availability.
type Availability interface {
	// SharesAvailable subjectively validates if Shares committed to the given Root are available on the Network.
	SharesAvailable(context.Context, *Root) error
}
