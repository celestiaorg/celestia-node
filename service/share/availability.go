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
const AvailabilityTimeout = 10 * time.Minute

// TODO: Implement BlockSync as Full Availability which downloads all the shares.
// TODO: Implement Cache Availability to store results of wrapped Validate execution in KVStore, so
//  (1) we don't re-execute successful DASes
// 	(2) we can retry failed DASes after some long period or somehow debug them
type Availability interface {
	SharesAvailable(context.Context, *Root) error
}
