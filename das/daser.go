package das

import (
	"context"
	"errors"
	"time"

	ipld "github.com/ipfs/go-ipld-format"
	logging "github.com/ipfs/go-log/v2"

	"github.com/celestiaorg/celestia-node/service/header"
	"github.com/celestiaorg/celestia-node/service/share"
)

var log = logging.Logger("das")

// ErrDASFailed is returned whenever DA sampling fails.
var ErrDASFailed = errors.New("das: sampling failed")

// das.Timeout specifies timeout for DA validation during which data have to be found on the network,
// otherwise ErrValidationFailed is thrown.
// TODO: https://github.com/celestiaorg/celestia-node/issues/10
const Timeout = 10 * time.Minute

// DefaultSamples sets the default amount of samples to be DASed.
var DefaultSamples = 16

type DASer interface {
	// DAS executes Data Availability Sampling over given DataAvailabilityHeader.
	// It randomly picks DefaultSamples amount of Block Shares and requests them from the network.
	// It fails with ErrDASFailed if didn't receive them for the Timeout.
	DAS(context.Context, *header.DataAvailabilityHeader) error
}

// NewDASer creates a new DASer.
func NewDASer(shares share.Service) DASer {
	return &daser{
		shares: shares,
	}
}

type daser struct {
	shares share.Service
}

func (das *daser) DAS(ctx context.Context, dah *header.DataAvailabilityHeader) error {
	log.Debugw("DAS", "dah", dah.Hash())
	samples, err := SampleSquare(len(dah.ColumnRoots), DefaultSamples)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(ctx, Timeout)
	defer cancel()

	errs := make(chan error, len(samples))
	for _, s := range samples {
		go func(s Sample) {
			_, err := das.shares.GetShare(ctx, dah, s.Row, s.Col)
			// we don't really care about Share bodies at this point
			select {
			case errs <- err:
			case <-ctx.Done():
			}
		}(s)
	}

	for range samples {
		var err error
		select {
		case err = <-errs:
		case <-ctx.Done():
			err = ctx.Err()
		}

		if err != nil {
			log.Errorw("DAS failed", "dah", dah.Hash(), "err", err)
			if errors.Is(err, ipld.ErrNotFound) || errors.Is(err, context.DeadlineExceeded) {
				return ErrDASFailed
			}

			return err
		}
	}

	return nil
}
