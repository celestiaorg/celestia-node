package ipld

import (
	"context"
	"encoding/hex"
	"errors"
	"sync"
	"time"

	"github.com/ipfs/go-blockservice"
	"github.com/ipfs/go-cid"
	format "github.com/ipfs/go-ipld-format"
	logging "github.com/ipfs/go-log/v2"
	"github.com/ipfs/go-merkledag"
	"github.com/tendermint/tendermint/pkg/da"
	"github.com/tendermint/tendermint/pkg/wrapper"

	"github.com/celestiaorg/celestia-node/ipld/plugin"
	"github.com/celestiaorg/nmt"
	"github.com/celestiaorg/rsmt2d"
)

var log = logging.Logger("ipld")

// RetrieveQuadrantTimeout limits the time for retrieval of a quadrant
// so that Retriever can retry another quadrant.
var RetrieveQuadrantTimeout = time.Minute * 5

// Retriever retrieves rsmt2d.ExtendedDataSquares from the IPLD network.
// Instead of requesting data 'share by share' it requests data by quadrants
// minimizing bandwidth usage in the happy cases.
//
//  ---- ----
// | 0  | 1  |
//  ---- ----
// | 2  | 3  |
//  ---- ----
// Retriever randomly picks one of the data square quadrants and tries to request them one by one until it is able to
// reconstruct the whole square.
type Retriever struct {
	dag blockservice.BlockService
}

// NewRetriever creates a new instance of the Retriever over IPLD Service and rmst2d.Codec
func NewRetriever(dag blockservice.BlockService) *Retriever {
	return &Retriever{dag: dag}
}

// Retrieve retrieves all the data committed to DataAvailabilityHeader.
// If not available locally, it aims to request from the network only 1/4 of the data and reconstructs other 3/4 parts
// using the rsmt2d.Codec. It steadily tries to request other 3/4 data if 1/4 is not found within
// RetrieveQuadrantTimeout or unrecoverable.
func (r *Retriever) Retrieve(ctx context.Context, dah *da.DataAvailabilityHeader) (*rsmt2d.ExtendedDataSquare, error) {
	log.Debugw("retrieving data square", "data_hash", hex.EncodeToString(dah.Hash()), "size", len(dah.RowsRoots))
	ses, err := r.newSession(ctx, dah)
	if err != nil {
		return nil, err
	}

	for _, q := range newQuadrants(dah) {
		eds, err := ses.retrieve(ctx, q)
		if err == nil {
			return eds, nil
		}

		var errByz *rsmt2d.ErrByzantineData
		if errors.As(err, &errByz) {
			return nil, NewErrByzantine(ctx, r.dag, dah, errByz)
		}
		log.Warnw("not enough shares to reconstruct data square, requesting more...", "err", err)
		// retry quadrants until we can reconstruct the EDS or error out
	}

	return nil, format.ErrNotFound
}

type retrieverSession struct {
	dag   blockservice.BlockGetter
	adder *NmtNodeAdder

	treeFn rsmt2d.TreeConstructorFn
	codec  rsmt2d.Codec

	dah            *da.DataAvailabilityHeader
	squareLk       sync.RWMutex
	square         [][]byte
	squareImported *rsmt2d.ExtendedDataSquare
}

func (r *Retriever) newSession(ctx context.Context, dah *da.DataAvailabilityHeader) (*retrieverSession, error) {
	size := len(dah.RowsRoots)
	adder := NewNmtNodeAdder(
		ctx,
		format.NewBatch(
			ctx,
			merkledag.NewDAGService(r.dag),
			format.MaxSizeBatchOption(batchSize(size)),
		),
	)
	ses := &retrieverSession{
		dag:   blockservice.NewSession(ctx, r.dag),
		adder: adder,
		treeFn: func() rsmt2d.Tree {
			tree := wrapper.NewErasuredNamespacedMerkleTree(uint64(size)/2, nmt.NodeVisitor(adder.Visit))
			return &tree
		},
		codec:  DefaultRSMT2DCodec(),
		dah:    dah,
		square: make([][]byte, size*size),
	}

	square, err := rsmt2d.ImportExtendedDataSquare(ses.square, ses.codec, ses.treeFn)
	if err != nil {
		return nil, err
	}

	ses.squareImported = square
	return ses, nil
}

func (rs *retrieverSession) retrieve(ctx context.Context, q *quadrant) (*rsmt2d.ExtendedDataSquare, error) {
	defer func() {
		// all shares which were requested or repaired are written to disk with the commit
		// we store *all*, so they are served to the network, including incorrectly committed data(BEFP case),
		// so that network can check BEFP
		if err := rs.adder.Commit(); err != nil {
			log.Errorw("committing DAG", "err", err)
		}
	}()
	// request quadrant and fill it into rs.square slice
	// we don't care about the errors here, just need to request as much data as we can to be able to reconstruct below
	rs.request(ctx, q)

	// try repair
	// TODO: Avoid reimporting of the square which can potentially remove the requirement for the lock
	rs.squareLk.Lock()
	defer rs.squareLk.Unlock()
	err := rs.squareImported.Repair(rs.dah.RowsRoots, rs.dah.ColumnRoots, rs.codec, rs.treeFn)
	if err != nil {
		return nil, err
	}

	log.Infow("data square reconstructed", "data_hash", hex.EncodeToString(rs.dah.Hash()), "size", len(rs.dah.RowsRoots))
	return rs.squareImported, nil
}

func (rs *retrieverSession) request(ctx context.Context, q *quadrant) {
	ctx, cancel := context.WithTimeout(ctx, RetrieveQuadrantTimeout)
	defer cancel()

	size := len(q.roots)
	wg := &sync.WaitGroup{}
	wg.Add(size)

	log.Debugw("requesting quadrant", "axis", q.source, "x", q.x, "y", q.y, "size", size)
	for i, root := range q.roots {
		go func(i int, root cid.Cid) {
			defer wg.Done()
			// get the root node
			nd, err := plugin.Get(ctx, rs.dag, root)
			if err != nil {
				return
			}
			// and go get shares of left or the right side of the whole col/row axis
			// the left or the right side of the tree represent some portion of the quadrant
			// which we put into the rs.square share-by-share by calculating shares' indexes using q.index
			GetShares(ctx, rs.dag, nd.Links()[q.x].Cid, size, func(j int, share Share) {
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
				}
				rs.squareLk.RUnlock()
			})
		}(i, root)
	}
	// wait for each root
	// we don't need to interrupt roots if one of them fails - downloading everything we can
	wg.Wait()
}
