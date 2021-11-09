package share

import (
	"context"
	"errors"

	format "github.com/ipfs/go-ipld-format"

	"github.com/celestiaorg/celestia-node/ipld"
)

// DefaultSamples sets the default amount of samples to be DASed by light Availability.
var DefaultSamples = 16

// lightAvailability implements Availability using Data Availability Sampling technic.
type lightAvailability struct {
	getter format.NodeGetter
}

// NewLightAvailability creates a new Light DataAvailability.
func NewLightAvailability(get format.NodeGetter) Availability {
	return &lightAvailability{
		getter: get,
	}
}

// SharesAvailable randomly samples DefaultSamples amount of Share committed to given Root.
// This way SharesAvailable subjectively verifies that Shares are available.
func (la *lightAvailability) SharesAvailable(ctx context.Context, dah *Root) error {
	log.Debugw("Validate availability", "root", dah.Hash())
	samples, err := SampleSquare(len(dah.ColumnRoots), DefaultSamples)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(ctx, AvailabilityTimeout)
	defer cancel()

	errs := make(chan error, len(samples))
	for _, s := range samples {
		go func(s Sample) {
			root, leaf := translate(dah, s.Row, s.Col)
			_, err := ipld.GetLeaf(ctx, la.getter, root, leaf, len(dah.RowsRoots))
			// we don't really care about Share bodies at this point
			// it also means we now saved the Share in local storage
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
			log.Errorw("availability validation failed", "root", dah.Hash(), "err", err)
			if errors.Is(err, format.ErrNotFound) || errors.Is(err, context.DeadlineExceeded) {
				return ErrNotAvailable
			}

			return err
		}
	}

	return nil
}
