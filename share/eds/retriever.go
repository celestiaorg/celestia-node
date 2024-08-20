package eds

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ipfs/boxo/blockservice"
	"github.com/ipfs/go-cid"
	logging "github.com/ipfs/go-log/v2"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"

	"github.com/celestiaorg/celestia-app/v2/pkg/wrapper"
	"github.com/celestiaorg/nmt"
	"github.com/celestiaorg/rsmt2d"

	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/celestia-node/share/eds/byzantine"
	"github.com/celestiaorg/celestia-node/share/ipld"
)

var (
	log    = logging.Logger("share/eds")
	tracer = otel.Tracer("share/eds")
)

// Retriever retrieves rsmt2d.ExtendedDataSquares from the IPLD network.
// Instead of requesting data 'share by share' it requests data by quadrants
// minimizing bandwidth usage in the happy cases.
//
//	 ---- ----
//	| 0  | 1  |
//	 ---- ----
//	| 2  | 3  |
//	 ---- ----
//
// Retriever randomly picks one of the data square quadrants and tries to request them one by one
// until it is able to reconstruct the whole square.
type Retriever struct {
	bServ blockservice.BlockService
}

// NewRetriever creates a new instance of the Retriever over IPLD BlockService and rmst2d.Codec
func NewRetriever(bServ blockservice.BlockService) *Retriever {
	return &Retriever{bServ: bServ}
}

// Retrieve retrieves all the data committed to DataAvailabilityHeader.
//
// If not available locally, it aims to request from the network only one quadrant (1/4) of the
// data square and reconstructs the other three quadrants (3/4). If the requested quadrant is not
// available within RetrieveQuadrantTimeout, it starts requesting another quadrant until either the
// data is reconstructed, context is canceled or ErrByzantine is generated.
func (r *Retriever) Retrieve(ctx context.Context, roots *share.AxisRoots) (*rsmt2d.ExtendedDataSquare, error) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel() // cancels all the ongoing requests if reconstruction succeeds early

	ctx, span := tracer.Start(ctx, "retrieve-square")
	defer span.End()
	span.SetAttributes(
		attribute.Int("size", len(roots.RowRoots)),
	)

	log.Debugw("retrieving data square", "data_hash", roots.String(), "size", len(roots.RowRoots))
	ses, err := r.newSession(ctx, roots)
	if err != nil {
		return nil, err
	}

	// wait for a signal to start reconstruction
	// try until either success or context or bad data
	for {
		select {
		case <-ses.Done():
			eds, err := ses.Reconstruct(ctx)
			if err == nil {
				ses.close(true)
				span.SetStatus(codes.Ok, "square-retrieved")
				return eds, nil
			}
			// check to ensure it is not a catastrophic ErrByzantine case, otherwise handle accordingly
			var errByz *rsmt2d.ErrByzantineData
			if errors.As(err, &errByz) {
				// session should be closed before constructing the Byzantine error to allow constructor to access
				// nmt proofs computed during the session
				ses.close(false)
				span.RecordError(err)
				return nil, byzantine.NewErrByzantine(ctx, r.bServ.Blockstore(), roots, errByz)
			}

			log.Warnw("not enough shares to reconstruct data square, requesting more...", "err", err)
		case <-ctx.Done():
			ses.close(false)
			return nil, ctx.Err()
		}
	}
}

// retrievalSession represents a data square retrieval session.
// It manages one data square that is being retrieved and
// quadrant request retries. Also, provides an API
// to reconstruct the block once enough shares are fetched.
type retrievalSession struct {
	roots *share.AxisRoots
	bget  blockservice.BlockGetter
	adder *ipld.NmtNodeAdder

	// TODO(@Wondertan): Extract into a separate data structure
	// https://github.com/celestiaorg/rsmt2d/issues/135
	squareQuadrants  []*quadrant
	squareCellsLks   [][]sync.Mutex
	squareCellsCount uint32
	squareSig        chan struct{}
	squareDn         chan struct{}
	squareLk         sync.RWMutex
	square           *rsmt2d.ExtendedDataSquare

	span trace.Span
}

// newSession creates a new retrieval session and kicks off requesting process.
func (r *Retriever) newSession(ctx context.Context, roots *share.AxisRoots) (*retrievalSession, error) {
	size := len(roots.RowRoots)

	adder := ipld.NewNmtNodeAdder(ctx, r.bServ, ipld.MaxSizeBatchOption(size))
	proofsVisitor := ipld.ProofsAdderFromCtx(ctx).VisitFn()
	visitor := func(hash []byte, children ...[]byte) {
		// use proofs adder if provided, to cache collected proofs while recomputing the eds
		if proofsVisitor != nil {
			proofsVisitor(hash, children...)
		}
		adder.Visit(hash, children...)
	}

	treeFn := func(_ rsmt2d.Axis, index uint) rsmt2d.Tree {
		tree := wrapper.NewErasuredNamespacedMerkleTree(uint64(size)/2, index, nmt.NodeVisitor(visitor))
		return &tree
	}

	square, err := rsmt2d.NewExtendedDataSquare(share.DefaultRSMT2DCodec(), treeFn, uint(size), share.Size)
	if err != nil {
		return nil, err
	}

	ses := &retrievalSession{
		roots:           roots,
		bget:            blockservice.NewSession(ctx, r.bServ),
		adder:           adder,
		squareQuadrants: newQuadrants(roots),
		squareCellsLks:  make([][]sync.Mutex, size),
		squareSig:       make(chan struct{}, 1),
		squareDn:        make(chan struct{}),
		square:          square,
		span:            trace.SpanFromContext(ctx),
	}
	for i := range ses.squareCellsLks {
		ses.squareCellsLks[i] = make([]sync.Mutex, size)
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
	err := rs.square.Repair(rs.roots.RowRoots, rs.roots.ColumnRoots)
	if err != nil {
		span.RecordError(err)
		return nil, err
	}
	log.Infow("data square reconstructed", "data_hash", rs.roots.String(), "size", len(rs.roots.RowRoots))
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

func (rs *retrievalSession) close(success bool) {
	defer rs.span.End()
	if success {
		return
	}
	// commit intermediate nodes to the blockservice if failed to reconstruct
	err := rs.adder.Commit()
	if err != nil {
		log.Warnw("failed to commit intermediate nodes", "err", err)
	}
}

// request kicks off quadrants requests.
// It instantly requests a quadrant and periodically requests more
// until either context is canceled or we are out of quadrants.
func (rs *retrievalSession) request(ctx context.Context) {
	t := time.NewTicker(RetrieveQuadrantTimeout)
	defer t.Stop()
	for retry := 0; retry < len(rs.squareQuadrants); retry++ {
		q := rs.squareQuadrants[retry]
		log.Debugw("requesting quadrant",
			"axis", q.source,
			"x", q.x,
			"y", q.y,
			"size", len(q.roots),
		)
		rs.span.AddEvent("requesting quadrant", trace.WithAttributes(
			attribute.Int("axis", int(q.source)),
			attribute.Int("x", q.x),
			attribute.Int("y", q.y),
			attribute.Int("size", len(q.roots)),
		))
		rs.doRequest(ctx, q)
		select {
		case <-t.C:
		case <-ctx.Done():
			return
		}
		log.Warnw("quadrant request timeout",
			"timeout", RetrieveQuadrantTimeout.String(),
			"axis", q.source,
			"x", q.x,
			"y", q.y,
			"size", len(q.roots),
		)
		rs.span.AddEvent("quadrant request timeout", trace.WithAttributes(
			attribute.Int("axis", int(q.source)),
			attribute.Int("x", q.x),
			attribute.Int("y", q.y),
			attribute.Int("size", len(q.roots)),
		))
	}
}

// doRequest requests the given quadrant by requesting halves of axis(Row or Col) using GetShares
// and fills shares into rs.square slice.
func (rs *retrievalSession) doRequest(ctx context.Context, q *quadrant) {
	size := len(q.roots)
	for i, root := range q.roots {
		go func(i int, root cid.Cid) {
			// get the root node
			nd, err := ipld.GetNode(ctx, rs.bget, root)
			if err != nil {
				rs.span.RecordError(err, trace.WithAttributes(
					attribute.Int("root-index", i),
				))
				return
			}
			// and go get shares of left or the right side of the whole col/row axis
			// the left or the right side of the tree represent some portion of the quadrant
			// which we put into the rs.square share-by-share by calculating shares' indexes using q.index
			ipld.GetShares(ctx, rs.bget, nd.Links()[q.x].Cid, size, func(j int, share share.Share) {
				// NOTE: Each share can appear twice here, for a Row and Col, respectively.
				// These shares are always equal, and we allow only the first one to be written
				// in the square.
				// NOTE-2: We may never actually fetch shares from the network *twice*.
				// Once a share is downloaded from the network it may be cached on the IPLD(blockservice) level.
				//
				// calc position of the share
				x, y := q.pos(i, j)
				// try to lock the share
				ok := rs.squareCellsLks[x][y].TryLock()
				if !ok {
					// if already locked and written - do nothing
					return
				}
				// The R lock here is *not* to protect rs.square from multiple
				// concurrent shares writes but to avoid races between share writes and
				// repairing attempts.
				// Shares are written atomically in their own slice slots and these "writes" do
				// not need synchronization!
				rs.squareLk.RLock()
				defer rs.squareLk.RUnlock()
				// the routine could be blocked above for some time during which the square
				// might be reconstructed, if so don't write anything and return
				if rs.isReconstructed() {
					return
				}
				if err := rs.square.SetCell(uint(x), uint(y), share); err != nil {
					// safe to ignore as:
					// * share size already verified
					// * the same share might come from either Row or Col
					return
				}
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
			})
		}(i, root)
	}
}
