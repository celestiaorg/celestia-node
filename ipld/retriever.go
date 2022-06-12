package ipld

import (
	"context"
	"encoding/hex"
	"errors"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ipfs/go-blockservice"
	"github.com/ipfs/go-cid"
	format "github.com/ipfs/go-ipld-format"
	logging "github.com/ipfs/go-log/v2"
	"github.com/tendermint/tendermint/pkg/da"
	"github.com/tendermint/tendermint/pkg/wrapper"

	"github.com/celestiaorg/celestia-node/ipld/plugin"
	"github.com/celestiaorg/nmt"
	"github.com/celestiaorg/rsmt2d"
)

var log = logging.Logger("ipld")

// Retriever retrieves rsmt2d.ExtendedDataSquares from the IPLD network.
// Instead of requesting data 'share by share' it requests data by quadrants
// minimizing bandwidth usage in the happy cases.
//
//  ---- ----
// | 0  | 1  |
//  ---- ----
// | 2  | 3  |
//  ---- ----
// Retriever randomly picks one of the data square quadrants and tries to request them one by one
// until it is able to reconstruct the whole square.
type Retriever struct {
	bServ blockservice.BlockService
}

// NewRetriever creates a new instance of the Retriever over IPLD Service and rmst2d.Codec
func NewRetriever(bServ blockservice.BlockService) *Retriever {
	return &Retriever{bServ: bServ}
}

// Retrieve retrieves all the data committed to DataAvailabilityHeader.
//
// If not available locally, it aims to request from the network only one quadrant (1/4) of the data square
// and reconstructs the other three quadrants (3/4). If the requested quadrant is not available within
// RetrieveQuadrantTimeout, it starts requesting another quadrant until either the data is
// reconstructed, context is canceled or ErrByzantine is generated.
func (r *Retriever) Retrieve(ctx context.Context, dah *da.DataAvailabilityHeader) (*rsmt2d.ExtendedDataSquare, error) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel() // cancels all the ongoing requests if reconstruction succeeds early

	log.Debugw("retrieving data square", "data_hash", hex.EncodeToString(dah.Hash()), "size", len(dah.RowsRoots))
	ses, err := r.newSession(ctx, dah)
	if err != nil {
		return nil, err
	}
	defer ses.Close()

	// wait for a signal to start reconstruction
	// try until either success or context or bad data
	for {
		select {
		case <-ses.Done():
			eds, err := ses.Reconstruct()
			if err == nil {
				return eds, nil
			}
			// check to ensure it is not a catastrophic ErrByzantine case, otherwise handle accordingly
			var errByz *rsmt2d.ErrByzantineData
			if errors.As(err, &errByz) {
				return nil, NewErrByzantine(ctx, r.bServ, dah, errByz)
			}
		case <-ctx.Done():
			return nil, ctx.Err()
		}

		var errByz *rsmt2d.ErrByzantineData
		if errors.As(err, &errByz) {
			return nil, NewErrByzantine(ctx, r.bServ, dah, errByz)
		}
		log.Warnw("not enough shares to reconstruct data square, requesting more...", "err", err)
		// retry quadrants until we can reconstruct the EDS or error out
	}
}

// retrievalSession represents a data square retrieval session.
// It manages one data square that is being retrieved and
// quadrant request retries. Also, provides an API
// to reconstruct the block once enough shares are fetched.
type retrievalSession struct {
	bget  blockservice.BlockGetter
	adder *NmtNodeAdder

	treeFn rsmt2d.TreeConstructorFn
	codec  rsmt2d.Codec

	dah            *da.DataAvailabilityHeader
	squareImported *rsmt2d.ExtendedDataSquare

	quadrants   []*quadrant
	sharesLks   []sync.Mutex
	sharesCount uint32

	squareLk sync.RWMutex
	square   [][]byte
	squareDn chan struct{}
}

// newSession creates a new retrieval session and kicks off requesting process.
func (r *Retriever) newSession(ctx context.Context, dah *da.DataAvailabilityHeader) (*retrievalSession, error) {
	size := len(dah.RowsRoots)
	adder := NewNmtNodeAdder(
		ctx,
		r.bServ,
		format.MaxSizeBatchOption(batchSize(size)),
	)
	ses := &retrievalSession{
		bget:  blockservice.NewSession(ctx, r.bServ),
		adder: adder,
		treeFn: func() rsmt2d.Tree {
			tree := wrapper.NewErasuredNamespacedMerkleTree(uint64(size)/2, nmt.NodeVisitor(adder.Visit))
			return &tree
		},
		codec:     DefaultRSMT2DCodec(),
		dah:       dah,
		quadrants: newQuadrants(dah),
		sharesLks: make([]sync.Mutex, size*size),
		square:    make([][]byte, size*size),
		squareDn:  make(chan struct{}, 1),
	}

	square, err := rsmt2d.ImportExtendedDataSquare(ses.square, ses.codec, ses.treeFn)
	if err != nil {
		return nil, err
	}

	ses.squareImported = square
	go ses.request(ctx)
	return ses, nil
}

// Done signals that enough shares have been retrieved to attempt
// square reconstruction. "Attempt" because there is no way currently to
// guarantee that reconstruction can be performed with the shares provided.
func (rs *retrievalSession) Done() <-chan struct{} {
	return rs.squareDn
}

// Reconstruct tries to reconstruct the data square and returns it on success.
func (rs *retrievalSession) Reconstruct() (*rsmt2d.ExtendedDataSquare, error) {
	// prevent further writes to the square
	rs.squareLk.Lock()
	defer rs.squareLk.Unlock()

	// TODO(@Wondertan): This is bad!
	//  * We should not reimport the square multiple times
	//  * We should set shares into imported square via SetShare(https://github.com/celestiaorg/rsmt2d/issues/83)
	//  to accomplish the above point.
	{
		squareImported, err := rsmt2d.ImportExtendedDataSquare(rs.square, rs.codec, rs.treeFn)
		if err != nil {
			return nil, err
		}
		rs.squareImported = squareImported
	}

	// and try to repair with what we have
	err := rs.squareImported.Repair(rs.dah.RowsRoots, rs.dah.ColumnRoots, rs.codec, rs.treeFn)
	if err != nil {
		return nil, err
	}
	log.Infow("data square reconstructed", "data_hash", hex.EncodeToString(rs.dah.Hash()), "size", len(rs.dah.RowsRoots))
	return rs.squareImported, nil
}

func (rs *retrievalSession) Close() error {
	// All shares which were requested or repaired are written to disk via `Commit`.
	// Note that we store *all*, so they are served to the network, including commit of incorrect
	// data(BEFP/ErrByzantineCase case), so that the network can check BEFP.
	err := rs.adder.Commit()
	if err != nil {
		log.Errorw("committing DAG", "err", err)
	}
	return err
}

// request kicks off quadrants requests.
// It instantly requests a quadrant and periodically requests more
// until either context is canceled or we are out of quadrants.
func (rs *retrievalSession) request(ctx context.Context) {
	t := time.NewTicker(RetrieveQuadrantTimeout)
	defer t.Stop()
	for retry := 0; retry < len(rs.quadrants); retry++ {
		q := rs.quadrants[retry]
		log.Debugw("requesting quadrant",
			"axis", q.source,
			"x", q.x,
			"y", q.y,
			"size", len(q.roots),
		)
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
	}
}

// doRequest requests the given quadrant by requesting halves of axis(Row or Col) using GetShares
// and fills shares into rs.square slice.
func (rs *retrievalSession) doRequest(ctx context.Context, q *quadrant) {
	size := len(q.roots)
	for i, root := range q.roots {
		go func(i int, root cid.Cid) {
			// get the root node
			nd, err := plugin.GetNode(ctx, rs.bget, root)
			if err != nil {
				return
			}
			// and go get shares of left or the right side of the whole col/row axis
			// the left or the right side of the tree represent some portion of the quadrant
			// which we put into the rs.square share-by-share by calculating shares' indexes using q.index
			GetShares(ctx, rs.bget, nd.Links()[q.x].Cid, size, func(j int, share Share) {
				// NOTE: Each share can be fetched twice, from two sources(Row or Col).
				// These shares are always equal, and we allow only the first one to be written
				// in the square.
				// NOTE-2: We never actually fetch shares from the network *twice*,
				// and they are cached on IPLD(blockservice) level.
				//
				// calc index of the share
				idx := q.index(i, j)
				// try to lock the share
				ok := rs.sharesLks[idx].TryLock()
				if !ok {
					// if already locked and written - do nothing
					return
				}
				// on success, write the share into the square
				rs.squareLk.RLock()
				// NOTE: The R lock here is *not* to protect rs.square from multiple
				// concurrent shares writes but to avoid races between share writes and
				// repairing attempts.
				// Shares are written atomically in their own slice slots and these "writes" do
				// not need synchronization!
				rs.square[idx] = share
				rs.squareLk.RUnlock()
				// if we have >= 1/4 of the square we can start trying to Reconstruct
				// TODO(@Wondertan): This is not an ideal way to know when to start
				//  reconstruction and can cause idle reconstruction tries in some cases,
				//  but it is totally fine for the happy case and for now.
				if atomic.AddUint32(&rs.sharesCount, 1) >= uint32(size*size) {
					select {
					case rs.squareDn <- struct{}{}:
					default:
					}
				}
			})
		}(i, root)
	}
}
