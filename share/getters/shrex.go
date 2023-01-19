package getters

import (
	"context"
	"errors"
	"github.com/celestiaorg/celestia-node/share/p2p/shrexeds"
	"github.com/celestiaorg/celestia-node/share/p2p/shrexnd"
	"github.com/libp2p/go-libp2p/core/peer"

	"github.com/celestiaorg/celestia-node/share"

	"github.com/celestiaorg/nmt/namespace"
	"github.com/celestiaorg/rsmt2d"
)

var _ share.Getter = (*ShrexGetter)(nil)
var errNoMorePeers = errors.New("all peers returned invalid responses")

type peerManager interface {
	next(ctx context.Context, root *share.Root) (peer.ID, error)
	remove(root *share.Root, peer peer.ID)
}

type ShrexGetter struct {
	edsClient *shrexeds.Client
	ndClient  *shrexnd.Client

	// for testing
	peers peerManager
}

func (sg *ShrexGetter) GetShare(ctx context.Context, root *share.Root, row, col int) (share.Share, error) {
	eds, err := sg.GetEDS(ctx, root)
	if eds != nil {
		return eds.GetCell(uint(row), uint(col)), nil
	}
	return nil, err
}

func (sg *ShrexGetter) GetEDS(ctx context.Context, root *share.Root) (*rsmt2d.ExtendedDataSquare, error) {
	for {
		to, err := sg.peers.next(ctx, root)
		if err != nil {
			return nil, err
		}

		eds, err := sg.edsClient.RequestEDS(ctx, root.Hash(), to)
		if eds != nil {
			return eds, nil
		}

		// non-nil error means the peer has misbehaved
		if err != nil {
			sg.peers.remove(root, to)
		}
	}
}

func (sg *ShrexGetter) GetSharesByNamespace(ctx context.Context, root *share.Root, id namespace.ID) (share.NamespacedShares, error) {
	for {
		to, err := sg.peers.next(ctx, root)
		if err != nil {
			return nil, err
		}

		eds, err := sg.ndClient.GetSharesByNamespace(ctx, root, id, to)
		if eds != nil {
			return eds, nil
		}

		// non-nil error means the peer has misbehaved
		if err != nil {
			sg.peers.remove(root, to)
		}
	}
}
