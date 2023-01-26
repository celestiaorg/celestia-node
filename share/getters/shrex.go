package getters

import (
	"context"
	"errors"

	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/celestia-node/share/p2p/peers"
	"github.com/celestiaorg/celestia-node/share/p2p/shrexeds"
	"github.com/celestiaorg/celestia-node/share/p2p/shrexnd"

	"github.com/celestiaorg/nmt/namespace"
	"github.com/celestiaorg/rsmt2d"
)

var _ share.Getter = (*ShrexGetter)(nil)

// ShrexGetter is a share.Getter that uses the shrex/eds and shrex/nd protocol to retrieve shares.
type ShrexGetter struct {
	edsClient *shrexeds.Client
	ndClient  *shrexnd.Client

	peers *peers.Manager
}

func (sg *ShrexGetter) GetShare(ctx context.Context, root *share.Root, row, col int) (share.Share, error) {
	return nil, errors.New("shrex-getter: GetShare is not supported")
}

func (sg *ShrexGetter) GetEDS(ctx context.Context, root *share.Root) (*rsmt2d.ExtendedDataSquare, error) {
	for {
		to, markSuccessful, err := sg.peers.GetPeer(ctx, root.Hash())
		if err != nil {
			return nil, err
		}

		eds, err := sg.edsClient.RequestEDS(ctx, root.Hash(), to)
		if eds != nil {
			markSuccessful(true)
			return eds, nil
		}

		// non-nil error means the peer has misbehaved
		if err != nil {
			markSuccessful(false)
		}
	}
}

func (sg *ShrexGetter) GetSharesByNamespace(
	ctx context.Context,
	root *share.Root,
	id namespace.ID,
) (share.NamespacedShares, error) {
	for {
		to, done, err := sg.peers.GetPeer(ctx, root.Hash())
		if err != nil {
			return nil, err
		}

		eds, err := sg.ndClient.RequestND(ctx, root, id, to)
		if eds != nil {
			return eds, nil
		}

		// non-nil error means the peer has misbehaved
		if err != nil {
			done(false)
		}
	}
}
