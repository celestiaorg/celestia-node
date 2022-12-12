package light

import (
	"context"
	"errors"
	"math"

	"github.com/libp2p/go-libp2p/core/peer"

	"github.com/ipfs/go-blockservice"
	ipldFormat "github.com/ipfs/go-ipld-format"
	logging "github.com/ipfs/go-log/v2"

	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/celestia-node/share/availability/discovery"
	shr "github.com/celestiaorg/celestia-node/share/service"
)

var log = logging.Logger("share/light")

// ShareAvailability implements share.Availability using Data Availability Sampling technique.
// It is light because it does not require the downloading of all the data to verify
// its availability. It is assumed that there are a lot of lightAvailability instances
// on the network doing sampling over the same Root to collectively verify its availability.
type ShareAvailability struct {
	shareServ shr.ShareService
	bserv     blockservice.BlockService
	// disc discovers new full nodes in the network.
	// it is not allowed to call advertise for light nodes (Full nodes only).
	disc   *discovery.Discovery
	cancel context.CancelFunc
}

// NewShareAvailability creates a new light Availability.
func NewShareAvailability(
	shareService shr.ShareService,
	bserv blockservice.BlockService,
	disc *discovery.Discovery,
) *ShareAvailability {
	la := &ShareAvailability{
		shareServ: shareService,
		bserv:     bserv,
		disc:      disc,
	}
	return la
}

func (la *ShareAvailability) Start(context.Context) error {
	ctx, cancel := context.WithCancel(context.Background())
	la.cancel = cancel

	go la.disc.EnsurePeers(ctx)

	// the share's service will be already started thanks
	// to fx lifecycle
	return nil
}

func (la *ShareAvailability) Stop(ctx context.Context) error {
	la.cancel()
	// the share's service will be already stopped thanks
	// to fx lifecycle
	return nil
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

	ctx, cancel := context.WithTimeout(ctx, share.AvailabilityTimeout)
	defer cancel()

	log.Debugw("starting sampling session", "root", dah.Hash())
	errs := make(chan error, len(samples))
	for _, s := range samples {
		go func(s Sample) {
			_, err := la.shareServ.GetShare(ctx, dah, s.Row, s.Col)
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
