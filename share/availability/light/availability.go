package light

import (
	"context"
	"errors"
	"math"

	ipldFormat "github.com/ipfs/go-ipld-format"
	logging "github.com/ipfs/go-log/v2"
	"github.com/libp2p/go-libp2p/core/peer"

	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/celestia-node/share/getters"
)

var log = logging.Logger("share/light")

// ShareAvailability implements share.Availability using Data Availability Sampling technique.
// It is light because it does not require the downloading of all the data to verify
// its availability. It is assumed that there are a lot of lightAvailability instances
// on the network doing sampling over the same Root to collectively verify its availability.
type ShareAvailability struct {
	getter share.Getter
}

// NewShareAvailability creates a new light Availability.
func NewShareAvailability(getter share.Getter) *ShareAvailability {
	return &ShareAvailability{getter}
}

// SharesAvailable randomly samples DefaultSampleAmount amount of Shares committed to the given
// Root. This way SharesAvailable subjectively verifies that Shares are available.
func (la *ShareAvailability) SharesAvailable(ctx context.Context, dah *share.Root, _ ...peer.ID) error {
	log.Debugw("Validate availability", "root", dah.Hash())
	// We assume the caller of this method has already performed basic validation on the
	// given dah/root. If for some reason this has not happened, the node should panic.
	if err := dah.ValidateBasic(); err != nil {
		log.Errorw("Availability validation cannot be performed on a malformed DataAvailabilityHeader",
			"err", err)
		panic(err)
	}
	samples, err := SampleSquare(len(dah.RowsRoots), DefaultSampleAmount)
	if err != nil {
		return err
	}

	// indicate to the share.Getter that a blockservice session should be created. This
	// functionality is optional and must be supported by the used share.Getter.
	ctx = getters.WithSession(ctx)
	ctx, cancel := context.WithTimeout(ctx, share.AvailabilityTimeout)
	defer cancel()

	log.Debugw("starting sampling session", "root", dah.Hash())
	errs := make(chan error, len(samples))
	for _, s := range samples {
		go func(s Sample) {
			log.Debugw("fetching share", "root", dah.Hash(), "row", s.Row, "col", s.Col)
			_, err := la.getter.GetShare(ctx, dah, s.Row, s.Col)
			if err != nil {
				log.Debugw("error fetching share", "root", dah.Hash(), "row", s.Row, "col", s.Col)
			}
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
			if !errors.Is(err, context.Canceled) {
				log.Errorw("availability validation failed", "root", dah.Hash(), "err", err)
			}
			if ipldFormat.IsNotFound(err) || errors.Is(err, context.DeadlineExceeded) {
				return share.ErrNotAvailable
			}

			return err
		}
	}

	return nil
}

// ProbabilityOfAvailability calculates the probability that the
// data square is available based on the amount of samples collected
// (DefaultSampleAmount).
//
// Formula: 1 - (0.75 ** amount of samples)
func (la *ShareAvailability) ProbabilityOfAvailability(context.Context) float64 {
	return 1 - math.Pow(0.75, float64(DefaultSampleAmount))
}
