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

// RetrieveQuadrantTimeout defines how much time Retriever waits before
// starting to retrieve another quadrant.
// TODO(@Wondertan): Its not yet clear what equilibrium time would be
//  and needs further experimentation.
var RetrieveQuadrantTimeout = time.Minute

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
// If not available locally, it aims to request from the network only any 1/4 of the data(quadrant)
// and reconstructs other 3/4 parts via rstm2d. If the requested quadrant is not available within
// RetrieveQuadrantTimeout, it starts requesting more, one by one, until either the data is
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
			// it might be a catastrophic error case, check for it
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

// retrievalSession represents a data square retrieval session
// It manages the data square, quadrant request retries
// and provides simple API to know when
type retrievalSession struct {
	bget  blockservice.BlockGetter
	adder *NmtNodeAdder

	treeFn rsmt2d.TreeConstructorFn
	codec  rsmt2d.Codec

	dah            *da.DataAvailabilityHeader
	squareImported *rsmt2d.ExtendedDataSquare

	quadrants []*quadrant
	squareLk  sync.RWMutex
	square    [][]byte
	shares    uint32
	done      chan struct{}
}

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
		square:    make([][]byte, size*size),
		done:      make(chan struct{}, 1),
	}

	square, err := rsmt2d.ImportExtendedDataSquare(ses.square, ses.codec, ses.treeFn)
	if err != nil {
		return nil, err
	}

	ses.squareImported = square
	go ses.request(ctx)
	return ses, nil
}

// Done notifies user to try reconstruction.
// Try because there is no way currently to guarantedd when
func (rs *retrievalSession) Done() <-chan struct{} {
	return rs.done
}

// Reconstruct tries to reconstruct the data square and returns it on success.
func (rs *retrievalSession) Reconstruct() (*rsmt2d.ExtendedDataSquare, error) {
	// prevent further writes to the square
	rs.squareLk.Lock()
	defer rs.squareLk.Unlock()
	// and try to repair with what we have
	err := rs.squareImported.Repair(rs.dah.RowsRoots, rs.dah.ColumnRoots, rs.codec, rs.treeFn)
	if err != nil {
		return nil, err
	}
	log.Debugw("data square reconstructed", "data_hash", hex.EncodeToString(rs.dah.Hash()), "size", len(rs.dah.RowsRoots))
	return rs.squareImported, nil
}

func (rs *retrievalSession) Close() error {
	// all shares which were requested or repaired are written to disk with the commit
	// we store *all*, so they are served to the network, including incorrectly committed
	// data(BEFP case), so that network can check BEFP
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
		// request quadrant and fill it into rs.square slice
		// we don't care about the errors here,
		// just need to request as much data as we can to be able to Reconstruct below
		rs.doRequest(ctx, rs.quadrants[retry])
		select {
		case <-t.C:
		case <-ctx.Done():
			return
		}
		log.Warnf("data quadrant wasn't received in %s, requesting another one...", RetrieveQuadrantTimeout.String())
	}
}

// doRequest requests the given quadrant by requesting halves of axis(Row or Col) using GetShares.
func (rs *retrievalSession) doRequest(ctx context.Context, q *quadrant) {
	size := len(q.roots)
	log.Debugw("requesting quadrant", "axis", q.source, "x", q.x, "y", q.y, "size", size)
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
				// the R lock here is *not* to protect rs.square from multiple concurrent shares writes
				// but to avoid races between share writes and repairing attempts
				// shares are written atomically in their own slice slot
				idx := q.index(i, j)
				rs.squareLk.RLock()
				// write only set nil shares, because shares can be passed here
				// twice for the same coordinate from row or column
				// NOTE: we never actually fetch the share from the network twice,
				//  and it is cached on IPLD(blockservice) level
				if rs.square[idx] == nil {
					rs.square[idx] = share
					// if we have >= 1/4 of the square we can start trying to Reconstruct
					// TODO(@Wondertan): This is not an ideal way to know when to start
					//  reconstruction and can cause idle reconstruction tries in some cases,
					//  but it is totally fine for the happy case and for now.
					if atomic.AddUint32(&rs.shares, 1) >= uint32(size*size) {
						select {
						case rs.done <- struct{}{}:
						default:
						}
					}
				}
				rs.squareLk.RUnlock()
			})
		}(i, root)
	}
}
