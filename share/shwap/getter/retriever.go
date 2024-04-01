package shwap_getter

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ipfs/boxo/blockservice"
	logging "github.com/ipfs/go-log/v2"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"

	"github.com/celestiaorg/celestia-app/pkg/wrapper"
	"github.com/celestiaorg/rsmt2d"

	"github.com/celestiaorg/celestia-node/header"
	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/celestia-node/share/eds/byzantine"
	"github.com/celestiaorg/celestia-node/share/shwap"
)

// TODO(@walldiss):
// - update comments
// - befp construction should work over share.getter instead of blockservice
// - use single bitswap session for fetching shares
// - don't request repaired shares
// - use enriched logger for session
// - remove per-share tracing
// - remove quadrants struct
// - remove unneeded locks
// - add metrics

var (
	log    = logging.Logger("share/eds")
	tracer = otel.Tracer("share/eds")
)

// edsRetriver retrieves rsmt2d.ExtendedDataSquares from the IPLD network.
// Instead of requesting data 'share by share' it requests data by quadrants
// minimizing bandwidth usage in the happy cases.
//
//	 ---- ----
//	| 0  | 1  |
//	 ---- ----
//	| 2  | 3  |
//	 ---- ----
//
// edsRetriver randomly picks one of the data square quadrants and tries to request them one by one
// until it is able to reconstruct the whole square.
type edsRetriver struct {
	bServ  blockservice.BlockService
	getter share.Getter
}

// newRetriever creates a new instance of the edsRetriver over IPLD BlockService and rmst2d.Codec
func newRetriever(getter *Getter) *edsRetriver {
	bServ := blockservice.New(getter.bstore, getter.fetch, blockservice.WithAllowlist(shwap.DefaultAllowlist))
	return &edsRetriver{
		bServ:  bServ,
		getter: getter,
	}
}

// Retrieve retrieves all the data committed to DataAvailabilityHeader.
//
// If not available locally, it aims to request from the network only one quadrant (1/4) of the
// data square and reconstructs the other three quadrants (3/4). If the requested quadrant is not
// available within RetrieveQuadrantTimeout, it starts requesting another quadrant until either the
// data is reconstructed, context is canceled or ErrByzantine is generated.
func (r *edsRetriver) Retrieve(ctx context.Context, h *header.ExtendedHeader) (*rsmt2d.ExtendedDataSquare, error) {
	dah := h.DAH
	ctx, cancel := context.WithCancel(ctx)
	defer cancel() // cancels all the ongoing requests if reconstruction succeeds early

	ctx, span := tracer.Start(ctx, "retrieve-square")
	defer span.End()
	span.SetAttributes(
		attribute.Int("size", len(dah.RowRoots)),
	)

	log.Debugw("retrieving data square", "data_hash", dah.String(), "size", len(dah.RowRoots))
	ses, err := r.newSession(ctx, h)
	if err != nil {
		return nil, err
	}
	defer ses.Close()

	// wait for a signal to start reconstruction
	// try until either success or context or bad data
	for {
		select {
		case <-ses.Done():
			eds, err := ses.Reconstruct(ctx)
			if err == nil {
				span.SetStatus(codes.Ok, "square-retrieved")
				return eds, nil
			}
			// check to ensure it is not a catastrophic ErrByzantine case, otherwise handle accordingly
			var errByz *rsmt2d.ErrByzantineData
			if errors.As(err, &errByz) {
				span.RecordError(err)
				return nil, byzantine.NewErrByzantine(ctx, r.bServ, dah, errByz)
			}

			log.Warnw("not enough shares to reconstruct data square, requesting more...", "err", err)
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
}

// retrievalSession represents a data square retrieval session.
// It manages one data square that is being retrieved and
// quadrant request retries. Also, provides an API
// to reconstruct the block once enough shares are fetched.
type retrievalSession struct {
	header *header.ExtendedHeader
	getter share.Getter

	squareCellsCount uint32
	squareSig        chan struct{}
	squareDn         chan struct{}
	squareLk         sync.RWMutex
	square           *rsmt2d.ExtendedDataSquare

	span trace.Span
}

// newSession creates a new retrieval session and kicks off requesting process.
func (r *edsRetriver) newSession(ctx context.Context, h *header.ExtendedHeader) (*retrievalSession, error) {
	size := len(h.DAH.RowRoots)

	treeFn := func(_ rsmt2d.Axis, index uint) rsmt2d.Tree {
		tree := wrapper.NewErasuredNamespacedMerkleTree(uint64(size)/2, index)
		return &tree
	}

	square, err := rsmt2d.NewExtendedDataSquare(share.DefaultRSMT2DCodec(), treeFn, uint(size), share.Size)
	if err != nil {
		return nil, err
	}

	ses := &retrievalSession{
		header:    h,
		getter:    r.getter,
		squareSig: make(chan struct{}, 1),
		squareDn:  make(chan struct{}),
		square:    square,
		span:      trace.SpanFromContext(ctx),
	}

	go ses.request(ctx)
	return ses, nil
}

// Done signals that enough shares have been retrieved to attempt
// square reconstruction. "Attempt" because there is no way currently to
// guarantee that reconstruction can be performed with the shares provided.
func (rs *retrievalSession) Done() <-chan struct{} {
	return rs.squareSig
}

// Reconstruct tries to reconstruct the data square and returns it on success.
func (rs *retrievalSession) Reconstruct(ctx context.Context) (*rsmt2d.ExtendedDataSquare, error) {
	if rs.isReconstructed() {
		return rs.square, nil
	}
	// prevent further writes to the square
	rs.squareLk.Lock()
	defer rs.squareLk.Unlock()

	_, span := tracer.Start(ctx, "reconstruct-square")
	defer span.End()

	// and try to repair with what we have
	err := rs.square.Repair(rs.header.DAH.RowRoots, rs.header.DAH.ColumnRoots)
	if err != nil {
		span.RecordError(err)
		return nil, err
	}
	log.Infow("data square reconstructed", "data_hash", rs.header.DAH.String(), "size", len(rs.header.DAH.RowRoots))
	close(rs.squareDn)
	return rs.square, nil
}

// isReconstructed report true whether the square attached to the session
// is already reconstructed.
func (rs *retrievalSession) isReconstructed() bool {
	select {
	case <-rs.squareDn:
		// return early if square is already reconstructed
		return true
	default:
		return false
	}
}

func (rs *retrievalSession) Close() error {
	defer rs.span.End()
	return nil
}

// request kicks off quadrants requests.
// It instantly requests a quadrant and periodically requests more
// until either context is canceled or we are out of quadrants.
func (rs *retrievalSession) request(ctx context.Context) {
	t := time.NewTicker(RetrieveQuadrantTimeout)
	defer t.Stop()
	for _, q := range newQuadrants() {
		log.Debugw("requesting quadrant",
			"x", q.x,
			"y", q.y,
		)
		rs.span.AddEvent("requesting quadrant", trace.WithAttributes(
			attribute.Int("x", q.x),
			attribute.Int("y", q.y),
		))
		rs.requestQuadrant(ctx, q)
		select {
		case <-t.C:
		case <-ctx.Done():
			return
		}
		log.Warnw("quadrant request timeout",
			"timeout", RetrieveQuadrantTimeout.String(),
			"x", q.x,
			"y", q.y,
		)
		rs.span.AddEvent("quadrant request timeout", trace.WithAttributes(
			attribute.Int("x", q.x),
			attribute.Int("y", q.y),
		))
	}
}

// requestQuadrant requests the given quadrant by requesting halves of axis(Row or Col) using GetShares
// and fills shares into rs.square slice.
func (rs *retrievalSession) requestQuadrant(ctx context.Context, q quadrant) {
	odsSize := len(rs.header.DAH.RowRoots) / 2
	for x := q.x * odsSize; x < (q.x+1)*odsSize; x++ {
		for y := q.y * odsSize; y < (q.y+1)*odsSize; y++ {
			go rs.requestCell(ctx, x, y)
		}
	}
}

func (rs *retrievalSession) requestCell(ctx context.Context, x, y int) {
	share, err := rs.getter.GetShare(ctx, rs.header, x, y)
	if err != nil {
		log.Debugw("failed to get share",
			"height", rs.header.Height,
			"x", x,
			"y", y,
			"err", err,
		)
		return
	}

	// the routine could be blocked above for some time during which the square
	// might be reconstructed, if so don't write anything and return
	if rs.isReconstructed() {
		return
	}

	rs.squareLk.RLock()
	defer rs.squareLk.RUnlock()

	if err := rs.square.SetCell(uint(x), uint(y), share); err != nil {
		log.Warnw("failed to set cell",
			"height", rs.header.Height,
			"x", x,
			"y", y,
			"err", err,
		)
		return
	}
	rs.indicateDone()
}

func (rs *retrievalSession) indicateDone() {
	size := len(rs.header.DAH.RowRoots) / 2
	// if we have >= 1/4 of the square we can start trying to Reconstruct
	// TODO(@Wondertan): This is not an ideal way to know when to start
	//  reconstruction and can cause idle reconstruction tries in some cases,
	//  but it is totally fine for the happy case and for now.
	//  The earlier we correctly know that we have the full square - the earlier
	//  we cancel ongoing requests - the less data is being wastedly transferred.
	if atomic.AddUint32(&rs.squareCellsCount, 1) >= uint32(size*size) {
		select {
		case rs.squareSig <- struct{}{}:
		default:
		}
	}
}
