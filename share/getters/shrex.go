package getters

import (
	"context"
	"errors"
	"fmt"
	"time"

	"go.uber.org/multierr"

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
	defaultMinRequestTimeout = time.Minute // should be >= shrexeds server write timeout
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
	var (
		attempt int
		err     error
	)
	for {
		attempt++
		start := time.Now()
		peer, setStatus, getErr := sg.peerManager.Peer(ctx, root.Hash())
		if getErr != nil {
			err = multierr.Append(err, getErr)
			log.Debugw("couldn't find peer",
				"datahash", root.String(),
				"err", getErr,
				"finished (s)", time.Since(start))
			return nil, fmt.Errorf("getter/shrex: %w", err)
		}

		reqStart := time.Now()
		reqCtx, cancel := ctxWithSplitTimeout(ctx, sg.minAttemptsCount-attempt+1, sg.minRequestTimeout)
		eds, getErr := sg.edsClient.RequestEDS(reqCtx, root.Hash(), peer)
		cancel()
		switch getErr {
		case nil:
			setStatus(peers.ResultSynced)
			return eds, nil
		case context.DeadlineExceeded:
		case p2p.ErrInvalidResponse:
			setStatus(peers.ResultBlacklistPeer)
		default:
			setStatus(peers.ResultCooldownPeer)
		}

		if !ErrorContains(err, getErr) {
			err = multierr.Append(err, getErr)
		}
		log.Debugw("request failed",
			"datahash", root.String(),
			"peer", peer.String(),
			"attempt", attempt,
			"err", getErr,
			"finished (s)", time.Since(reqStart))
	}
}

func (sg *ShrexGetter) GetSharesByNamespace(
	ctx context.Context,
	root *share.Root,
	id namespace.ID,
) (share.NamespacedShares, error) {
	var (
		attempt int
		err     error
	)
	for {
		attempt++
		start := time.Now()
		peer, setStatus, getErr := sg.peerManager.Peer(ctx, root.Hash())
		if getErr != nil {
			err = multierr.Append(err, getErr)
			log.Debugw("couldn't find peer",
				"datahash", root.String(),
				"err", getErr,
				"finished (s)", time.Since(start))
			return nil, fmt.Errorf("getter/shrex: %w", err)
		}

		reqStart := time.Now()
		reqCtx, cancel := ctxWithSplitTimeout(ctx, sg.minAttemptsCount-attempt+1, sg.minRequestTimeout)
		nd, getErr := sg.ndClient.RequestND(reqCtx, root, id, peer)
		cancel()
		switch getErr {
		case nil:
			setStatus(peers.ResultSuccess)
			return nd, nil
		case context.DeadlineExceeded:
		case p2p.ErrInvalidResponse:
			setStatus(peers.ResultBlacklistPeer)
		default:
			setStatus(peers.ResultCooldownPeer)
		}

		if !ErrorContains(err, getErr) {
			err = multierr.Append(err, getErr)
		}
		log.Debugw("request failed",
			"datahash", root.String(),
			"peer", peer.String(),
			"attempt", attempt,
			"err", getErr,
			"finished (s)", time.Since(reqStart))
	}
}
