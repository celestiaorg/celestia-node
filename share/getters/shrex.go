package getters

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/celestiaorg/nmt/namespace"
	"github.com/celestiaorg/rsmt2d"

	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/celestia-node/share/p2p"
	"github.com/celestiaorg/celestia-node/share/p2p/peers"
	"github.com/celestiaorg/celestia-node/share/p2p/shrexeds"
	"github.com/celestiaorg/celestia-node/share/p2p/shrexnd"
)

var _ share.Getter = (*ShrexGetter)(nil)

const (
	// defaultMinRequestTimeout value is set according to observed time taken by healthy peer to
	// serve getEDS request for block size 256
	defaultMinRequestTimeout = time.Second * 10
	defaultMinAttemptsCount  = 3
)

// ShrexGetter is a share.Getter that uses the shrex/eds and shrex/nd protocol to retrieve shares.
type ShrexGetter struct {
	edsClient *shrexeds.Client
	ndClient  *shrexnd.Client

	peerManager *peers.Manager

	// minRequestTimeout limits minimal timeout given to single peer by getter for serving the request.
	minRequestTimeout time.Duration
	// minAttemptsCount will be used to split request timeout into multiple attempts. It will allow to
	// attempt multiple peers in scope of one request before context timeout is reached
	minAttemptsCount int
}

func NewShrexGetter(edsClient *shrexeds.Client, ndClient *shrexnd.Client, peerManager *peers.Manager) *ShrexGetter {
	return &ShrexGetter{
		edsClient:         edsClient,
		ndClient:          ndClient,
		peerManager:       peerManager,
		minRequestTimeout: defaultMinRequestTimeout,
		minAttemptsCount:  defaultMinAttemptsCount,
	}
}

func (sg *ShrexGetter) Start(ctx context.Context) error {
	return sg.peerManager.Start(ctx)
}

func (sg *ShrexGetter) Stop(ctx context.Context) error {
	return sg.peerManager.Stop(ctx)
}

func (sg *ShrexGetter) GetShare(ctx context.Context, root *share.Root, row, col int) (share.Share, error) {
	return nil, errors.New("getter/shrex: GetShare is not supported")
}

func (sg *ShrexGetter) GetEDS(ctx context.Context, root *share.Root) (*rsmt2d.ExtendedDataSquare, error) {
	for {
		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("getter/shrex: %w", ctx.Err())
		default:
		}
		peer, setStatus, err := sg.peerManager.Peer(ctx, root.Hash())
		if err != nil {
			log.Debugw("couldn't find peer", "datahash", root.String(), "err", err)
			return nil, fmt.Errorf("getter/shrex: %w", err)
		}

		reqCtx, cancel := ctxWithSplitTimeout(ctx, sg.minAttemptsCount, sg.minRequestTimeout)
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
			return nil, fmt.Errorf("getter/shrex: %w", ctx.Err())
		default:
		}
		peer, setStatus, err := sg.peerManager.Peer(ctx, root.Hash())
		if err != nil {
			log.Debugw("couldn't find peer", "datahash", root.String(), "err", err)
			return nil, fmt.Errorf("getter/shrex: %w", err)
		}

		reqCtx, cancel := ctxWithSplitTimeout(ctx, sg.minAttemptsCount, sg.minRequestTimeout)
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
