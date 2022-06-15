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

// DefaultSampleAmount sets the default amount of samples to be sampled from the network by lightAvailability.
var DefaultSampleAmount = 16

// LightAvailability implements Availability using Data Availability Sampling technique.
// It is light because it does not require the downloading of all the data to verify
// its availability. It is assumed that there are a lot of lightAvailability instances
// on the network doing sampling over the same Root to collectively verify its availability.
type LightAvailability struct {
	bserv      blockservice.BlockService
	notifee    *disc.Notifee
	discoverer discovery.Discoverer

	ctx    context.Context
	cancel context.CancelFunc
}

// NewLightAvailability creates a new light Availability.
func NewLightAvailability(bserv blockservice.BlockService, d discovery.Discoverer, host host.Host) *LightAvailability {
	la := &LightAvailability{
		bserv:      bserv,
		notifee:    disc.NewNotifee(disc.NewLimitedSet(disc.PeersLimit), host),
		discoverer: d,
	}
	return la
}

// SharesAvailable randomly samples DefaultSamples amount of Shares committed to the given Root.
// This way SharesAvailable subjectively verifies that Shares are available.
func (la *LightAvailability) SharesAvailable(ctx context.Context, dah *Root) error {
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

	ctx, cancel := context.WithTimeout(ctx, AvailabilityTimeout)
	defer cancel()

	ses := blockservice.NewSession(ctx, la.bserv)
	errs := make(chan error, len(samples))
	for _, s := range samples {
		go func(s Sample) {
			root, leaf := translate(dah, s.Row, s.Col)
			_, err := ipld.GetShare(ctx, ses, root, leaf, len(dah.RowsRoots))
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
			if errors.Is(err, format.ErrNotFound) || errors.Is(err, context.DeadlineExceeded) {
				return ErrNotAvailable
			}

			return err
		}
	}

	return nil
}

// Start starts looking for a new peers in network.
func (la *LightAvailability) Start(context.Context) error {
	la.ctx, la.cancel = context.WithCancel(context.Background())
	disc.FindPeers(la.ctx, la.discoverer, la.notifee)
	return nil
}

// Stop stops all discovery processes.
func (la *LightAvailability) Stop(context.Context) error {
	la.cancel()
	return nil
}
