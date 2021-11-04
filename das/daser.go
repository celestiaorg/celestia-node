package das

import (
	"context"
	"errors"
	"time"

	"github.com/celestiaorg/celestia-node/service/header"
	"github.com/celestiaorg/celestia-node/service/share"
)

// DASTimeout specifies timeout for DA validation during which data have to be found on the network,
// otherwise ErrValidationFailed is thrown.
// TODO: https://github.com/celestiaorg/celestia-node/issues/10
const DASTimeout = 10 * time.Minute

const DefaultSamples = 15

// ErrDASFailed is returned whenever DA sampling fails.
var ErrDASFailed = errors.New("das: sampling failed")

type DASer interface {
	DAS(context.Context, *header.DataAvailabilityHeader) error
}

type daser struct {
	shares share.Service
}

func (s *daser) DAS(ctx context.Context, dah *header.DataAvailabilityHeader) error {

}
