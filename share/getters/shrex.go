package getters

import (
	"context"
	"errors"
	"time"

	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/celestia-node/share/p2p"
	"github.com/celestiaorg/celestia-node/share/p2p/peers"
	"github.com/celestiaorg/celestia-node/share/p2p/shrexeds"
	"github.com/celestiaorg/celestia-node/share/p2p/shrexnd"

	"github.com/celestiaorg/nmt/namespace"
	"github.com/celestiaorg/rsmt2d"
)

var _ share.Getter = (*ShrexGetter)(nil)

const defaultMaxRequestDuration = time.Second * 10

// ShrexGetter is a share.Getter that uses the shrex/eds and shrex/nd protocol to retrieve shares.
type ShrexGetter struct {
	edsClient *shrexeds.Client
	ndClient  *shrexnd.Client

	peerManager        *peers.Manager
	maxRequestDuration time.Duration
}

func NewShrexGetter(edsClient *shrexeds.Client, ndClient *shrexnd.Client, peerManager *peers.Manager) *ShrexGetter {
	return &ShrexGetter{
		edsClient:          edsClient,
		ndClient:           ndClient,
		peerManager:        peerManager,
		maxRequestDuration: defaultMaxRequestDuration,
	}
}

func (sg *ShrexGetter) Start(ctx context.Context) error {
	return sg.peerManager.Start(ctx)
}

func (sg *ShrexGetter) Stop(ctx context.Context) error {
	return sg.peerManager.Stop(ctx)
}

func (sg *ShrexGetter) GetShare(ctx context.Context, root *share.Root, row, col int) (share.Share, error) {
	return nil, errors.New("shrex-getter: GetShare is not supported")
}

func (sg *ShrexGetter) GetEDS(ctx context.Context, root *share.Root) (*rsmt2d.ExtendedDataSquare, error) {
	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}
		peer, setStatus, err := sg.peerManager.Peer(ctx, root.Hash())
		if err != nil {
			log.Debugw("couldn't find peer", "datahash", root.String(), "err", err)
			return nil, err
		}

		reqCtx, cancel := context.WithTimeout(ctx, sg.maxRequestDuration)
		eds, err := sg.edsClient.RequestEDS(reqCtx, root.Hash(), peer)
		cancel()
		switch err {
		case nil:
			setStatus(peers.ResultSynced)
			return eds, nil
		case context.DeadlineExceeded:
			log.Debugw("request exceeded deadline, trying with new peer", "datahash", root.String())
		case p2p.ErrInvalidResponse:
			setStatus(peers.ResultBlacklistPeer)
		default:
			setStatus(peers.ResultCooldownPeer)
		}
	}
}

func (sg *ShrexGetter) GetSharesByNamespace(
	ctx context.Context,
	root *share.Root,
	id namespace.ID,
) (share.NamespacedShares, error) {
	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}
		peer, setStatus, err := sg.peerManager.Peer(ctx, root.Hash())
		if err != nil {
			log.Debugw("couldn't find peer", "datahash", root.String(), "err", err)
			return nil, err
		}

		reqCtx, cancel := context.WithTimeout(ctx, sg.maxRequestDuration)
		nd, err := sg.ndClient.RequestND(reqCtx, root, id, peer)
		cancel()
		switch err {
		case nil:
			setStatus(peers.ResultSuccess)
			return nd, nil
		case context.DeadlineExceeded:
			log.Debugw("request exceeded deadline, trying with new peer", "datahash", root.String())
		case p2p.ErrInvalidResponse:
			setStatus(peers.ResultBlacklistPeer)
		default:
			setStatus(peers.ResultCooldownPeer)
		}
	}
}
