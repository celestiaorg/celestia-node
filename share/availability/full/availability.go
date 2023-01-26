package full

import (
	"context"
	"errors"

	ipldFormat "github.com/ipfs/go-ipld-format"
	logging "github.com/ipfs/go-log/v2"
	"github.com/libp2p/go-libp2p/core/peer"

	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/celestia-node/share/availability/discovery"
)

var log = logging.Logger("share/full")

// ShareAvailability implements share.Availability using the full data square
// recovery technique. It is considered "full" because it is required
// to download enough shares to fully reconstruct the data square.
type ShareAvailability struct {
	getter share.Getter
	disc   *discovery.Discovery

	cancel context.CancelFunc
}

// NewShareAvailability creates a new full ShareAvailability.
func NewShareAvailability(getter share.Getter, disc *discovery.Discovery) *ShareAvailability {
	return &ShareAvailability{
		getter: getter,
		disc:   disc,
	}
}

func (fa *ShareAvailability) Start(context.Context) error {
	ctx, cancel := context.WithCancel(context.Background())
	fa.cancel = cancel

	go fa.disc.Advertise(ctx)
	fa.disc.EnsurePeers(ctx)
	return nil
}

func (fa *ShareAvailability) Stop(context.Context) error {
	fa.cancel()
	return nil
}

// SharesAvailable reconstructs the data committed to the given Root by requesting
// enough Shares from the network.
func (fa *ShareAvailability) SharesAvailable(ctx context.Context, root *share.Root, _ ...peer.ID) error {
	ctx, cancel := context.WithTimeout(ctx, share.AvailabilityTimeout)
	defer cancel()
	// we assume the caller of this method has already performed basic validation on the
	// given dah/root. If for some reason this has not happened, the node should panic.
	if err := root.ValidateBasic(); err != nil {
		log.Errorw("Availability validation cannot be performed on a malformed DataAvailabilityHeader",
			"err", err)
		panic(err)
	}

	_, err := fa.getter.GetEDS(ctx, root)
	if err != nil {
		log.Errorw("availability validation failed", "root", root.Hash(), "err", err)
		if ipldFormat.IsNotFound(err) || errors.Is(err, context.DeadlineExceeded) {
			return share.ErrNotAvailable
		}

		return err
	}
	return err
}

func (fa *ShareAvailability) ProbabilityOfAvailability(context.Context) float64 {
	return 1
}
