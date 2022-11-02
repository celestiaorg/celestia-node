package share

import (
	"context"
	"errors"
	"time"

	"github.com/celestiaorg/celestia-app/pkg/da"
)

// ErrNotAvailable is returned whenever DA sampling fails.
var ErrNotAvailable = errors.New("share: data not available")

const (
	// AvailabilityTimeout specifies timeout for DA validation during which data have to be found on the network,
	// otherwise ErrNotAvailable is fired.
	// TODO: https://github.com/celestiaorg/celestia-node/issues/10
	DefaultAvailabilityTimeout = 20 * time.Minute

	// SampleAmount specifies the minimum required amount of samples a light node must perform
	// before declaring that a block is available
	DefaultSampleAmount = 16

	// DefaultWriteBatchSize defines the size of the batched header write.
	// Headers are written in batches not to thrash the underlying Datastore with writes.
	// TODO(@Wondertan, @renaynay): Those values must be configurable
	// and proper defaults should be set for specific node type. (#709)
	DefaultWriteBatchSize = 2048

	// DefaultCacheAvailabilityPrefix defines the string key to be used with in
	// the datastore as the cache availability data store key
	DefaultCacheAvailabilityPrefix = "sampling_result"
)

// Root represents root commitment to multiple Shares.
// In practice, it is a commitment to all the Data in a square.
type Root = da.DataAvailabilityHeader

// Availability defines interface for validation of Shares' availability.
type Availability interface {
	// Parameterizable allows the implemeters of Availability to be configurable/parameterizable
	Parameterizable

	// SharesAvailable subjectively validates if Shares committed to the given Root are available on the Network.
	SharesAvailable(context.Context, *Root) error
	// ProbabilityOfAvailability calculates the probability of the data square
	// being available based on the number of samples collected.
	// TODO(@Wondertan): Merge with SharesAvailable method, eventually
	ProbabilityOfAvailability() float64
}
