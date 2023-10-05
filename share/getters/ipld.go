package getters

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"

	"github.com/ipfs/boxo/blockservice"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"

	"github.com/celestiaorg/rsmt2d"

	"github.com/celestiaorg/celestia-node/libs/utils"
	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/celestia-node/share/eds"
	"github.com/celestiaorg/celestia-node/share/eds/byzantine"
	"github.com/celestiaorg/celestia-node/share/ipld"
)

var _ share.Getter = (*IPLDGetter)(nil)

var ipldMeter = otel.Meter("share/getters/ipld")

type ipldMetrics struct {
	edsAttempts   metric.Int64Histogram
	shareAttempts metric.Int64Histogram
}

func (m *ipldMetrics) recordEDSAttempt(ctx context.Context, attemptCount int, success bool) {
	if m == nil {
		return
	}
	if ctx.Err() != nil {
		ctx = context.Background()
	}
	m.edsAttempts.Record(ctx, int64(attemptCount),
		metric.WithAttributes(
			attribute.Bool("success", success)))
}

func (m *ipldMetrics) recordShareAttempt(ctx context.Context, attemptCount int, success bool) {
	if m == nil {
		return
	}
	if ctx.Err() != nil {
		ctx = context.Background()
	}
	m.shareAttempts.Record(ctx, int64(attemptCount),
		metric.WithAttributes(
			attribute.Bool("success", success)))
}

// IPLDGetter is a share.Getter that retrieves shares from the bitswap network. Result caching is
// handled by the provided blockservice. A blockservice session will be created for retrieval if the
// passed context is wrapped with WithSession.
type IPLDGetter struct {
	rtrv  *eds.Retriever
	bServ blockservice.BlockService

	metrics *ipldMetrics
}

// NewIPLDGetter creates a new share.Getter that retrieves shares from the bitswap network.
func NewIPLDGetter(bServ blockservice.BlockService) *IPLDGetter {
	return &IPLDGetter{
		rtrv:  eds.NewRetriever(bServ),
		bServ: bServ,
	}
}

func (ig *IPLDGetter) WithMetrics() error {
	edsAttemptHistogram, err := ipldMeter.Int64Histogram(
		"getters_ipld_eds_attempts_per_request",
		metric.WithDescription("Number of attempts per ipld/eds request"),
	)
	if err != nil {
		return err
	}

	shareAttemptHistogram, err := ipldMeter.Int64Histogram(
		"getters_ipld_share_attempts_per_request",
		metric.WithDescription("Number of attempts per ipld/share request"),
	)
	if err != nil {
		return err
	}

	ig.metrics = &ipldMetrics{
		edsAttempts:   edsAttemptHistogram,
		shareAttempts: shareAttemptHistogram,
	}
	return nil
}

// GetShare gets a single share at the given EDS coordinates from the bitswap network.
func (ig *IPLDGetter) GetShare(ctx context.Context, dah *share.Root, row, col int) (share.Share, error) {
	var err error
	ctx, span := tracer.Start(ctx, "ipld/get-share", trace.WithAttributes(
		attribute.Int("row", row),
		attribute.Int("col", col),
	))
	defer func() {
		utils.SetStatusAndEnd(span, err)
	}()

	upperBound := len(dah.RowRoots)
	if row >= upperBound || col >= upperBound {
		err := share.ErrOutOfBounds
		span.RecordError(err)
		return nil, err
	}
	root, leaf := ipld.Translate(dah, row, col)

	// wrap the blockservice in a session if it has been signaled in the context.
	blockGetter := getGetter(ctx, ig.bServ)
	s, err := ipld.GetShare(ctx, blockGetter, root, leaf, len(dah.RowRoots))
	if errors.Is(err, ipld.ErrNodeNotFound) {
		// convert error to satisfy getter interface contract
		err = share.ErrNotFound
	}

	ig.metrics.recordShareAttempt(ctx, 1, err != nil)
	if err != nil {
		return nil, fmt.Errorf("getter/ipld: failed to retrieve share: %w", err)
	}

	return s, nil
}

func (ig *IPLDGetter) GetEDS(ctx context.Context, root *share.Root) (eds *rsmt2d.ExtendedDataSquare, err error) {
	ctx, span := tracer.Start(ctx, "ipld/get-eds")
	defer func() {
		utils.SetStatusAndEnd(span, err)
	}()

	// rtrv.Retrieve calls shares.GetShares until enough shares are retrieved to reconstruct the EDS
	eds, err = ig.rtrv.Retrieve(ctx, root)
	if errors.Is(err, ipld.ErrNodeNotFound) {
		// convert error to satisfy getter interface contract
		err = share.ErrNotFound
	}
	var errByz *byzantine.ErrByzantine
	if errors.As(err, &errByz) {
		ig.metrics.recordEDSAttempt(ctx, 1, false)
		return nil, err
	}
	if err != nil {
		ig.metrics.recordEDSAttempt(ctx, 1, false)
		return nil, fmt.Errorf("getter/ipld: failed to retrieve eds: %w", err)
	}

	ig.metrics.recordEDSAttempt(ctx, 1, true)
	return eds, nil
}

func (ig *IPLDGetter) GetSharesByNamespace(
	ctx context.Context,
	root *share.Root,
	namespace share.Namespace,
) (shares share.NamespacedShares, err error) {
	ctx, span := tracer.Start(ctx, "ipld/get-shares-by-namespace", trace.WithAttributes(
		attribute.String("namespace", namespace.String()),
	))
	defer func() {
		utils.SetStatusAndEnd(span, err)
	}()

	if err = namespace.ValidateForData(); err != nil {
		return nil, err
	}

	// wrap the blockservice in a session if it has been signaled in the context.
	blockGetter := getGetter(ctx, ig.bServ)
	shares, err = collectSharesByNamespace(ctx, blockGetter, root, namespace)
	if errors.Is(err, ipld.ErrNodeNotFound) {
		// convert error to satisfy getter interface contract
		err = share.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("getter/ipld: failed to retrieve shares by namespace: %w", err)
	}
	return shares, nil
}

var sessionKey = &session{}

// session is a struct that can optionally be passed by context to the share.Getter methods using
// WithSession to indicate that a blockservice session should be created.
type session struct {
	sync.Mutex
	atomic.Pointer[blockservice.Session]
	ctx context.Context
}

// WithSession stores an empty session in the context, indicating that a blockservice session should
// be created.
func WithSession(ctx context.Context) context.Context {
	return context.WithValue(ctx, sessionKey, &session{ctx: ctx})
}

func getGetter(ctx context.Context, service blockservice.BlockService) blockservice.BlockGetter {
	s, ok := ctx.Value(sessionKey).(*session)
	if !ok {
		return service
	}

	val := s.Load()
	if val != nil {
		return val
	}

	s.Lock()
	defer s.Unlock()
	val = s.Load()
	if val == nil {
		val = blockservice.NewSession(s.ctx, service)
		s.Store(val)
	}
	return val
}
