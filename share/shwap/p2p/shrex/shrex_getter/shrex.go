package shrex_getter

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"time"

	logging "github.com/ipfs/go-log/v2"
	libpeer "github.com/libp2p/go-libp2p/core/peer"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"

	libshare "github.com/celestiaorg/go-square/v3/share"
	"github.com/celestiaorg/rsmt2d"

	"github.com/celestiaorg/celestia-node/header"
	"github.com/celestiaorg/celestia-node/libs/utils"
	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/celestia-node/share/availability"
	"github.com/celestiaorg/celestia-node/share/eds"
	"github.com/celestiaorg/celestia-node/share/shwap"
	"github.com/celestiaorg/celestia-node/share/shwap/p2p/shrex"
	"github.com/celestiaorg/celestia-node/share/shwap/p2p/shrex/peers"
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

// Getter is a share.Getter that uses the shrex protocol to retrieve shares.
type Getter struct {
	client *shrex.Client

	fullPeerManager     *peers.Manager
	archivalPeerManager *peers.Manager

	// minRequestTimeout limits minimal timeout given to single peer by getter for serving the request.
	minRequestTimeout time.Duration
	// minAttemptsCount will be used to split request timeout into multiple attempts. It will allow to
	// attempt multiple peers in scope of one request before context timeout is reached
	minAttemptsCount int

	availabilityWindow time.Duration

	metrics *metrics
}

func NewGetter(
	client *shrex.Client,
	fullPeerManager *peers.Manager,
	archivalManager *peers.Manager,
	availWindow time.Duration,
) *Getter {
	s := &Getter{
		client:              client,
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

func (sg *Getter) GetSamples(
	ctx context.Context,
	header *header.ExtendedHeader,
	coords []shwap.SampleCoords,
) ([]shwap.Sample, error) {
	var err error
	ctx, span := tracer.Start(ctx, "shrex/get-eds")
	defer func() {
		utils.SetStatusAndEnd(span, err)
	}()
	// short circuit if the data root is empty
	if header.DAH.Equals(share.EmptyEDSRoots()) {
		return []shwap.Sample{}, nil
	}

	requests := make([]shwap.SampleID, len(coords))
	for i, coord := range coords {
		sID, err := shwap.NewSampleID(header.Height(), coord, len(header.DAH.RowRoots))
		if err != nil {
			return nil, fmt.Errorf("failed to create a sampleID from the coordinates: %w", err)
		}
		requests[i] = sID
	}

	samples := make([]shwap.Sample, len(requests))
	errGroup, ctx := errgroup.WithContext(ctx)
	errGroup.SetLimit(len(requests))

	for i, request := range requests {
		logger := log.With(
			"source", "shrex_getter",
			"request_type", request.Name(),
			"hash", header.DAH.String(),
			"rowIndex", request.RowIndex,
			"colIndex", request.ShareIndex,
		)
		errGroup.Go(func() error {
			req := func(ctx context.Context, peer libpeer.ID) (int64, error) {
				return sg.client.Get(ctx, &request, &samples[i], peer)
			}
			verify := func() error {
				if samples[i].IsEmpty() {
					return errors.New("nil response")
				}
				return samples[i].Verify(header.DAH, request.RowIndex, request.ShareIndex)
			}
			err = sg.executeRequest(ctx, logger, header, request.Name(), req, verify)
			if err != nil {
				return err
			}
			return nil
		})
	}
	if err = errGroup.Wait(); err != nil {
		return nil, err
	}
	return samples, nil
}

func (sg *Getter) GetRow(ctx context.Context, header *header.ExtendedHeader, rowIndex int) (shwap.Row, error) {
	var err error
	ctx, span := tracer.Start(ctx, "shrex/get-eds")
	defer func() {
		utils.SetStatusAndEnd(span, err)
	}()
	// short circuit if the data root is empty
	if header.DAH.Equals(share.EmptyEDSRoots()) {
		return shwap.Row{}, nil
	}

	request, err := shwap.NewRowID(header.Height(), rowIndex, len(header.DAH.RowRoots))
	if err != nil {
		return shwap.Row{}, err
	}

	response := shwap.Row{}

	logger := log.With(
		"source", "shrex_getter",
		"request_type", request.Name(),
		"hash", header.DAH.String(),
		"rowIndex", rowIndex,
	)

	req := func(ctx context.Context, peer libpeer.ID) (int64, error) {
		return sg.client.Get(ctx, &request, &response, peer)
	}

	verify := func() error {
		if response.IsEmpty() {
			return errors.New("nil response")
		}
		return response.Verify(header.DAH, rowIndex)
	}

	err = sg.executeRequest(ctx, logger, header, request.Name(), req, verify)
	if err != nil {
		return shwap.Row{}, err
	}

	return response, nil
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

	request, err := shwap.NewEdsID(header.Height())
	if err != nil {
		return nil, err
	}

	var (
		buff     = bytes.NewBuffer(make([]byte, 0))
		response = &eds.Rsmt2D{}
	)

	logger := log.With(
		"source", "shrex_getter",
		"request_type", request.Name(),
		"hash", header.DAH.String(),
	)

	req := func(ctx context.Context, peer libpeer.ID) (int64, error) {
		return sg.client.Get(ctx, &request, buff, peer)
	}

	build := func() error {
		if buff.Len() == 0 {
			return errors.New("nil response")
		}
		response, err = eds.ReadAccessor(ctx, buff, header.DAH)
		return err
	}

	err = sg.executeRequest(ctx, logger, header, request.Name(), req, build)
	if err != nil {
		return nil, err
	}
	return response.ExtendedDataSquare, err
}

func (sg *Getter) GetNamespaceData(
	ctx context.Context,
	header *header.ExtendedHeader,
	namespace libshare.Namespace,
) (shwap.NamespaceData, error) {
	err := namespace.ValidateForData()
	if err != nil {
		return nil, err
	}

	ctx, span := tracer.Start(ctx, "shrex/get-shares-by-namespace", trace.WithAttributes(
		attribute.String("namespace", namespace.String()),
	))
	defer func() {
		utils.SetStatusAndEnd(span, err)
	}()

	// verify that the namespace could exist inside the roots before starting network requests
	dah := header.DAH
	rowIdxs, err := share.RowsWithNamespace(dah, namespace)
	if err != nil {
		return nil, err
	}
	if len(rowIdxs) == 0 {
		return shwap.NamespaceData{}, nil
	}

	request, err := shwap.NewNamespaceDataID(header.Height(), namespace)
	if err != nil {
		return nil, err
	}
	response := shwap.NamespaceData{}

	logger := log.With(
		"source", "shrex_getter",
		"request_type", request.Name(),
		"hash", dah.String(),
		"namespace", namespace.String(),
	)

	req := func(ctx context.Context, peer libpeer.ID) (int64, error) {
		return sg.client.Get(ctx, &request, &response, peer)
	}

	verify := func() error {
		if response == nil {
			return errors.New("nil response")
		}
		return response.Verify(dah, namespace)
	}

	err = sg.executeRequest(ctx, logger, header, request.Name(), req, verify)
	if err != nil {
		return nil, err
	}

	return response, nil
}

func (sg *Getter) GetRangeNamespaceData(
	ctx context.Context,
	header *header.ExtendedHeader,
	from, to int,
) (shwap.RangeNamespaceData, error) {
	var err error
	ctx, span := tracer.Start(ctx, "shrex/get-range-namespace-data", trace.WithAttributes(
		attribute.Int64("height", int64(header.Height())),
		attribute.Int("from", from),
		attribute.Int("to", to),
	))
	defer func() {
		utils.SetStatusAndEnd(span, err)
	}()

	edsID, err := shwap.NewEdsID(header.Height())
	if err != nil {
		return shwap.RangeNamespaceData{}, err
	}

	request, err := shwap.NewRangeNamespaceDataID(edsID, from, to, len(header.DAH.RowRoots)/2)
	if err != nil {
		return shwap.RangeNamespaceData{}, err
	}

	response := shwap.RangeNamespaceData{}

	logger := log.With(
		"source", "shrex_getter",
		"request_type", request.Name(),
		"hash", header.DAH.String(),
		"from", from,
		"to", to,
	)

	req := func(ctx context.Context, peer libpeer.ID) (int64, error) {
		return sg.client.Get(ctx, &request, &response, peer)
	}

	verify := func() error {
		if response.IsEmpty() {
			return errors.New("nil response")
		}

		fromCoords, err := shwap.SampleCoordsFrom1DIndex(from, len(header.DAH.RowRoots)/2)
		if err != nil {
			return err
		}
		// `to-1` to make an inclusive coordinate of the range
		toCoords, err := shwap.SampleCoordsFrom1DIndex(to-1, len(header.DAH.RowRoots)/2)
		if err != nil {
			return err
		}

		return response.VerifyInclusion(
			fromCoords,
			toCoords,
			len(header.DAH.RowRoots)/2,
			header.DAH.RowRoots[fromCoords.Row:toCoords.Row+1])
	}

	err = sg.executeRequest(ctx, logger, header, request.Name(), req, verify)
	if err != nil {
		return shwap.RangeNamespaceData{}, err
	}

	return response, nil
}

func (sg *Getter) getPeer(
	ctx context.Context,
	header *header.ExtendedHeader,
) (libpeer.ID, peers.DoneFunc, error) {
	if !availability.IsWithinWindow(header.Time(), sg.availabilityWindow) {
		p, df, err := sg.archivalPeerManager.Peer(ctx, header.DAH.Hash(), header.Height())
		return p, df, err
	}
	return sg.fullPeerManager.Peer(ctx, header.DAH.Hash(), header.Height())
}

// requestFn defines a function type that wraps the actual request logic as a closure.
// The closure captures additional request parameters and then executes
// the request with the provided context and peer ID.
// It returns the length of data that was read and an error
type requestFn func(context.Context, libpeer.ID) (int64, error)

// handleFn defines a function type that wraps response handling logic as a closure.
// The closure captures the response data and validation parameters performing the verification.
type handleFn func() error

// executeRequest performs a request operation with automatic peer management
// and retry logic. It orchestrates the complete request lifecycle including peer selection,
// request execution, response handling, and peer scoring management.
//
// The method continuously attempts to find suitable peers and execute requests until
// successful or context cancellation.
func (sg *Getter) executeRequest(
	ctx context.Context,
	logger *zap.SugaredLogger,
	extHeader *header.ExtendedHeader,
	reqType string,
	req requestFn,
	handle handleFn,
) error {
	var (
		attempt int
		err     error
	)

	for {
		if ctx.Err() != nil {
			sg.metrics.recordAttempts(ctx, reqType, attempt, false)
			return errors.Join(err, ctx.Err())
		}
		attempt++
		start := time.Now()

		peer, setStatus, getErr := sg.getPeer(ctx, extHeader)
		if getErr != nil {
			logger.Debugw("couldn't find peer",
				"err", getErr,
				"finished (s)", time.Since(start))
			sg.metrics.recordAttempts(ctx, reqType, attempt, false)
			return errors.Join(err, getErr)
		}

		reqStart := time.Now()
		reqCtx, cancel := utils.CtxWithSplitTimeout(ctx, sg.minAttemptsCount-attempt+1, sg.minRequestTimeout)

		_, getErr = req(reqCtx, peer)
		cancel()
		switch {
		case getErr == nil:
			setStatus(peers.ResultNoop)
			sg.metrics.recordAttempts(ctx, reqType, attempt, true)
			verifyErr := handle()
			if verifyErr != nil {
				getErr = verifyErr
				setStatus(peers.ResultBlacklistPeer)
				break
			}
			setStatus(peers.ResultNoop)
			return nil
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
		logger.Debugw("request failed",
			"peer", peer.String(),
			"attempt", attempt,
			"err", getErr,
			"finished (s)", time.Since(reqStart),
		)
	}
}
