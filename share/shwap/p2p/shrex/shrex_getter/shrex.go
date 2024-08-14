package shrex_getter //nolint:revive,stylecheck // underscore in pkg name will be fixed with shrex refactoring

import (
	"context"
	"errors"
	"fmt"
	"time"

	logging "github.com/ipfs/go-log/v2"
	libpeer "github.com/libp2p/go-libp2p/core/peer"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"

	"github.com/celestiaorg/rsmt2d"

	"github.com/celestiaorg/celestia-node/header"
	"github.com/celestiaorg/celestia-node/libs/utils"
	"github.com/celestiaorg/celestia-node/pruner"
	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/celestia-node/share/shwap"
	"github.com/celestiaorg/celestia-node/share/shwap/p2p/shrex"
	"github.com/celestiaorg/celestia-node/share/shwap/p2p/shrex/peers"
	"github.com/celestiaorg/celestia-node/share/shwap/p2p/shrex/shrexeds"
	"github.com/celestiaorg/celestia-node/share/shwap/p2p/shrex/shrexnd"
)

var (
	tracer = otel.Tracer("shrex/getter")
	meter  = otel.Meter("shrex/getter")
	log    = logging.Logger("shrex/getter")
)

var _ shwap.Getter = (*Getter)(nil)

const (
	// defaultMinRequestTimeout value is set according to observed time taken by healthy peer to
	// serve getEDS request for block size 256
	defaultMinRequestTimeout = time.Minute // should be >= shrexeds server write timeout
	defaultMinAttemptsCount  = 3
)

type metrics struct {
	edsAttempts metric.Int64Histogram
	ndAttempts  metric.Int64Histogram
}

func (m *metrics) recordEDSAttempt(ctx context.Context, attemptCount int, success bool) {
	if m == nil {
		return
	}
	ctx = utils.ResetContextOnError(ctx)
	m.edsAttempts.Record(ctx, int64(attemptCount),
		metric.WithAttributes(
			attribute.Bool("success", success)))
}

func (m *metrics) recordNDAttempt(ctx context.Context, attemptCount int, success bool) {
	if m == nil {
		return
	}
	ctx = utils.ResetContextOnError(ctx)
	m.ndAttempts.Record(ctx, int64(attemptCount),
		metric.WithAttributes(
			attribute.Bool("success", success)))
}

func (sg *Getter) WithMetrics() error {
	edsAttemptHistogram, err := meter.Int64Histogram(
		"getters_shrex_eds_attempts_per_request",
		metric.WithDescription("Number of attempts per shrex/eds request"),
	)
	if err != nil {
		return err
	}

	ndAttemptHistogram, err := meter.Int64Histogram(
		"getters_shrex_nd_attempts_per_request",
		metric.WithDescription("Number of attempts per shrex/nd request"),
	)
	if err != nil {
		return err
	}

	sg.metrics = &metrics{
		edsAttempts: edsAttemptHistogram,
		ndAttempts:  ndAttemptHistogram,
	}
	return nil
}

// Getter is a share.Getter that uses the shrex/eds and shrex/nd protocol to retrieve shares.
type Getter struct {
	edsClient *shrexeds.Client
	ndClient  *shrexnd.Client

	fullPeerManager     *peers.Manager
	archivalPeerManager *peers.Manager

	// minRequestTimeout limits minimal timeout given to single peer by getter for serving the request.
	minRequestTimeout time.Duration
	// minAttemptsCount will be used to split request timeout into multiple attempts. It will allow to
	// attempt multiple peers in scope of one request before context timeout is reached
	minAttemptsCount int

	availabilityWindow pruner.AvailabilityWindow

	metrics *metrics
}

func NewGetter(
	edsClient *shrexeds.Client,
	ndClient *shrexnd.Client,
	fullPeerManager *peers.Manager,
	archivalManager *peers.Manager,
	availWindow pruner.AvailabilityWindow,
) *Getter {
	s := &Getter{
		edsClient:           edsClient,
		ndClient:            ndClient,
		fullPeerManager:     fullPeerManager,
		archivalPeerManager: archivalManager,
		minRequestTimeout:   defaultMinRequestTimeout,
		minAttemptsCount:    defaultMinAttemptsCount,
		availabilityWindow:  availWindow,
	}

	return s
}

func (sg *Getter) Start(ctx context.Context) error {
	err := sg.fullPeerManager.Start(ctx)
	if err != nil {
		return err
	}
	return sg.archivalPeerManager.Start(ctx)
}

func (sg *Getter) Stop(ctx context.Context) error {
	err := sg.fullPeerManager.Stop(ctx)
	if err != nil {
		return err
	}
	return sg.archivalPeerManager.Stop(ctx)
}

func (sg *Getter) GetShare(context.Context, *header.ExtendedHeader, int, int) (share.Share, error) {
	return nil, fmt.Errorf("getter/shrex: GetShare %w", shwap.ErrOperationNotSupported)
}

func (sg *Getter) GetEDS(ctx context.Context, header *header.ExtendedHeader) (*rsmt2d.ExtendedDataSquare, error) {
	var err error
	ctx, span := tracer.Start(ctx, "shrex/get-eds")
	defer func() {
		utils.SetStatusAndEnd(span, err)
	}()

	// short circuit if the data root is empty
	if header.DAH.Equals(share.EmptyEDSRoots()) {
		return share.EmptyEDS(), nil
	}

	var attempt int
	for {
		if ctx.Err() != nil {
			sg.metrics.recordEDSAttempt(ctx, attempt, false)
			return nil, errors.Join(err, ctx.Err())
		}
		attempt++
		start := time.Now()

		peer, setStatus, getErr := sg.getPeer(ctx, header)
		if getErr != nil {
			log.Debugw("eds: couldn't find peer",
				"hash", header.DAH.String(),
				"err", getErr,
				"finished (s)", time.Since(start))
			sg.metrics.recordEDSAttempt(ctx, attempt, false)
			return nil, errors.Join(err, getErr)
		}

		reqStart := time.Now()
		reqCtx, cancel := utils.CtxWithSplitTimeout(ctx, sg.minAttemptsCount-attempt+1, sg.minRequestTimeout)
		eds, getErr := sg.edsClient.RequestEDS(reqCtx, header.DAH, header.Height(), peer)
		cancel()
		switch {
		case getErr == nil:
			setStatus(peers.ResultNoop)
			sg.metrics.recordEDSAttempt(ctx, attempt, true)
			return eds, nil
		case errors.Is(getErr, context.DeadlineExceeded),
			errors.Is(getErr, context.Canceled):
			setStatus(peers.ResultCooldownPeer)
		case errors.Is(getErr, shrex.ErrNotFound):
			getErr = shwap.ErrNotFound
			setStatus(peers.ResultCooldownPeer)
		case errors.Is(getErr, shrex.ErrInvalidResponse):
			setStatus(peers.ResultBlacklistPeer)
		default:
			setStatus(peers.ResultCooldownPeer)
		}

		if !shrex.ErrorContains(err, getErr) {
			err = errors.Join(err, getErr)
		}
		log.Debugw("eds: request failed",
			"hash", header.DAH.String(),
			"peer", peer.String(),
			"attempt", attempt,
			"err", getErr,
			"finished (s)", time.Since(reqStart))
	}
}

func (sg *Getter) GetSharesByNamespace(
	ctx context.Context,
	header *header.ExtendedHeader,
	namespace share.Namespace,
) (shwap.NamespaceData, error) {
	if err := namespace.ValidateForData(); err != nil {
		return nil, err
	}
	var (
		attempt int
		err     error
	)
	ctx, span := tracer.Start(ctx, "shrex/get-shares-by-namespace", trace.WithAttributes(
		attribute.String("namespace", namespace.String()),
	))
	defer func() {
		utils.SetStatusAndEnd(span, err)
	}()

	// verify that the namespace could exist inside the roots before starting network requests
	dah := header.DAH
	rowIdxs := share.RowsWithNamespace(dah, namespace)
	if len(rowIdxs) == 0 {
		return shwap.NamespaceData{}, nil
	}

	for {
		if ctx.Err() != nil {
			sg.metrics.recordNDAttempt(ctx, attempt, false)
			return nil, errors.Join(err, ctx.Err())
		}
		attempt++
		start := time.Now()

		peer, setStatus, getErr := sg.getPeer(ctx, header)
		if getErr != nil {
			log.Debugw("nd: couldn't find peer",
				"hash", dah.String(),
				"namespace", namespace.String(),
				"err", getErr,
				"finished (s)", time.Since(start))
			sg.metrics.recordNDAttempt(ctx, attempt, false)
			return nil, errors.Join(err, getErr)
		}

		reqStart := time.Now()
		reqCtx, cancel := utils.CtxWithSplitTimeout(ctx, sg.minAttemptsCount-attempt+1, sg.minRequestTimeout)
		nd, getErr := sg.ndClient.RequestND(reqCtx, header.Height(), namespace, peer)
		cancel()
		switch {
		case getErr == nil:
			// both inclusion and non-inclusion cases needs verification
			if verErr := nd.Validate(dah, namespace); verErr != nil {
				getErr = verErr
				setStatus(peers.ResultBlacklistPeer)
				break
			}
			setStatus(peers.ResultNoop)
			sg.metrics.recordNDAttempt(ctx, attempt, true)
			return nd, nil
		case errors.Is(getErr, context.DeadlineExceeded),
			errors.Is(getErr, context.Canceled):
			setStatus(peers.ResultCooldownPeer)
		case errors.Is(getErr, shrex.ErrNotFound):
			getErr = shwap.ErrNotFound
			setStatus(peers.ResultCooldownPeer)
		case errors.Is(getErr, shrex.ErrInvalidResponse):
			setStatus(peers.ResultBlacklistPeer)
		default:
			setStatus(peers.ResultCooldownPeer)
		}

		if !shrex.ErrorContains(err, getErr) {
			err = errors.Join(err, getErr)
		}
		log.Debugw("nd: request failed",
			"hash", dah.String(),
			"namespace", namespace.String(),
			"peer", peer.String(),
			"attempt", attempt,
			"err", getErr,
			"finished (s)", time.Since(reqStart))
	}
}

func (sg *Getter) getPeer(
	ctx context.Context,
	header *header.ExtendedHeader,
) (libpeer.ID, peers.DoneFunc, error) {
	if !pruner.IsWithinAvailabilityWindow(header.Time(), sg.availabilityWindow) {
		p, df, err := sg.archivalPeerManager.Peer(ctx, header.DAH.Hash(), header.Height())
		return p, df, err
	}
	return sg.fullPeerManager.Peer(ctx, header.DAH.Hash(), header.Height())
}
