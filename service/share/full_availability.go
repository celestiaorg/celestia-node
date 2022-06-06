package share

import (
	"context"
	"errors"

	"github.com/ipfs/go-blockservice"
	format "github.com/ipfs/go-ipld-format"
	"github.com/libp2p/go-libp2p-core/discovery"

	"github.com/libp2p/go-libp2p-core/host"

	"github.com/celestiaorg/celestia-node/ipld"
	disc "github.com/celestiaorg/celestia-node/service/share/discovery"
)

// fullAvailability implements Availability using the full data square
// recovery technique. It is considered "full" because it is required
// to download enough shares to fully reconstruct the data square.
type fullAvailability struct {
	notifee *disc.Notifee

	rtrv    *ipld.Retriever
	service discovery.Discovery

	ctx    context.Context
	cancel context.CancelFunc
}

// NewFullAvailability creates a new full Availability.
func NewFullAvailability(bServ blockservice.BlockService, d discovery.Discovery, host host.Host) *fullAvailability {
	fa := &fullAvailability{
		notifee: disc.NewNotifee(disc.NewPeerCache(), host),
		rtrv:    ipld.NewRetriever(bServ),
		service: d,
	}

	return fa
}

// SharesAvailable reconstructs the data committed to the given Root by requesting
// enough Shares from the network.
func (fa *fullAvailability) SharesAvailable(ctx context.Context, root *Root) error {
	ctx, cancel := context.WithTimeout(ctx, AvailabilityTimeout)
	defer cancel()
	// we assume the caller of this method has already performed basic validation on the
	// given dah/root. If for some reason this has not happened, the node should panic.
	if err := root.ValidateBasic(); err != nil {
		log.Errorw("Availability validation cannot be performed on a malformed DataAvailabilityHeader",
			"err", err)
		panic(err)
	}

	_, err := fa.rtrv.Retrieve(ctx, root)
	if err != nil {
		log.Errorw("availability validation failed", "root", root.Hash(), "err", err)
		if errors.Is(err, format.ErrNotFound) || errors.Is(err, context.DeadlineExceeded) {
			return ErrNotAvailable
		}

		return err
	}
	return err
}

// Start announces to the network and then starts looking for new peers
func (fa *fullAvailability) Start(context.Context) error {
	fa.ctx, fa.cancel = context.WithCancel(context.Background())
	disc.Advertise(fa.ctx, fa.service)
	disc.FindPeers(fa.ctx, fa.service, fa.notifee)
	return nil
}

// Stop cancels all discovery processes.
func (fa *fullAvailability) Stop(context.Context) error {
	fa.cancel()
	return nil
}
