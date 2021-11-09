package share

import (
	"context"
	"errors"

	ipld "github.com/ipfs/go-ipld-format"
)

// DefaultSamples sets the default amount of samples to be DASed by light Availability.
var DefaultSamples = 16

type lightAvailability struct {
	shares Service
}

// NewLightAvailability creates a new Light DataAvailability.
func NewLightAvailability(shares Service) Availability {
	return &lightAvailability{
		shares: shares,
	}
}

func (das *lightAvailability) SharesAvailable(ctx context.Context, dah *Root) error {
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
			_, err := das.shares.GetShare(ctx, dah, s.Row, s.Col)
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
			if errors.Is(err, ipld.ErrNotFound) || errors.Is(err, context.DeadlineExceeded) {
				return ErrNotAvailable
			}

			return err
		}
	}

	return nil
}
