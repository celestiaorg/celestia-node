package ipld

import (
	"context"
	"encoding/hex"
	"sync"
	"time"

	"github.com/ipfs/go-cid"
	format "github.com/ipfs/go-ipld-format"
	logging "github.com/ipfs/go-log/v2"
	"github.com/ipfs/go-merkledag"
	"github.com/tendermint/tendermint/pkg/da"
	"github.com/tendermint/tendermint/pkg/wrapper"

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
// Retriever randomly picks one of the data square quadrants and tries to request them one by one until it is able
// the whole square.
type Retriever struct {
	dag   format.DAGService
	codec rsmt2d.Codec
}

// NewRetriever creates a new instance of the Retriever over IPLD Service and rmst2d.Codec
func NewRetriever(dag format.DAGService, codec rsmt2d.Codec) *Retriever {
	return &Retriever{dag: dag, codec: codec}
}

// Retrieve retrieves all the data committed to DataAvailabilityHeader.
// It aims to request from the network only 1/4 of the data and reconstructs other 3/4 parts using the rsmt2d.Codec.
// It steadily tries to request other 3/4 data if 1/4 is not found within RetrieveQuadrantTimeout or unrecoverable.
func (r *Retriever) Retrieve(ctx context.Context, dah *da.DataAvailabilityHeader) (*rsmt2d.ExtendedDataSquare, error) {
	ses := r.newSession(ctx, dah)
	for _, qs := range newQuadrants(dah) {
		// TODO: ErrByzantineRow/Col
		for _, q := range qs {
			eds, err := ses.retrieve(ctx, q)
			if err == nil {
				return eds, nil
			}
		}
		// retry quadrants until we can reconstruct the EDS or error out
	}

	return nil, format.ErrNotFound
}

type retrieverSession struct {
	dag   format.NodeGetter
	adder *NmtNodeAdder

	treeFn rsmt2d.TreeConstructorFn
	codec  rsmt2d.Codec

	dah    *da.DataAvailabilityHeader
	square [][]byte
}

func (r *Retriever) newSession(ctx context.Context, dah *da.DataAvailabilityHeader) *retrieverSession {
	size := len(dah.RowsRoots)
	adder := NewNmtNodeAdder(ctx, format.NewBatch(ctx, r.dag, format.MaxSizeBatchOption(batchSize(size))))
	return &retrieverSession{
		dag:   merkledag.NewSession(ctx, r.dag),
		adder: adder,
		treeFn: func() rsmt2d.Tree {
			tree := wrapper.NewErasuredNamespacedMerkleTree(uint64(size)/2, nmt.NodeVisitor(adder.Visit))
			return &tree
		},
		codec:  r.codec,
		dah:    dah,
		square: make([][]byte, size*size),
	}
}

func (rs *retrieverSession) retrieve(ctx context.Context, q *quadrant) (*rsmt2d.ExtendedDataSquare, error) {
	log.Debugw("retrieving data square", "data_hash", hex.EncodeToString(rs.dah.Hash()), "size", len(rs.square))
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
	err := rsmt2d.RepairExtendedDataSquare(rs.dah.RowsRoots, rs.dah.ColumnRoots, rs.square, rs.codec, rs.treeFn)
	if err != nil {
		log.Warnw("not enough shares to reconstruct data square, requesting more...", "err", err)
		return nil, err
	}

	log.Infow("data square reconstructed", "data_hash", hex.EncodeToString(rs.dah.Hash()), "size", len(rs.square))
	return rsmt2d.ImportExtendedDataSquare(rs.square, rs.codec, rs.treeFn)
}

func (rs *retrieverSession) request(ctx context.Context, q *quadrant) {
	ctx, cancel := context.WithTimeout(ctx, RetrieveQuadrantTimeout)
	defer cancel()

	size := len(q.roots)
	wg := &sync.WaitGroup{}
	wg.Add(size)

	log.Debugw("requesting quadrant", "x", q.x, "y", q.y, "len(roots)", size)
	for i, root := range q.roots {
		go func(i int, root cid.Cid) {
			defer wg.Done()
			nd, err := rs.dag.Get(ctx, root)
			if err != nil {
				log.Errorw("getting root", "root", root.String(), "err", err)
				return
			}
			// TODO(@Wondertan): GetLeaves should return everything it was able to request even on error,
			// 	so that we fill as much data as possible
			// get leaves of left or right subtree
			nds, err := GetLeaves(ctx, rs.dag, nd.Links()[q.x].Cid, make([]format.Node, 0, size))
			if err != nil {
				log.Errorw("getting all the leaves", "root", root.String(), "err", err)
				return
			}
			// fill leaves into the square
			for j, nd := range nds {
				rs.square[q.index(i, j)] = nd.RawData()[1+NamespaceSize:]
			}
		}(i, root)
	}
	// wait for each root
	// we don't need to interrupt roots if one of them fails - downloading everything we can
	wg.Wait()
}
