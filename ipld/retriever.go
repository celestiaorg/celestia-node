package ipld

import (
	"context"
	"math/rand"
	"time"

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
// TODO(@Wondertan): Randomize requesting from Col Roots with retrying.
func (r *Retriever) Retrieve(ctx context.Context, dah *da.DataAvailabilityHeader) (*rsmt2d.ExtendedDataSquare, error) {
	daroots := dah.RowsRoots
	size, origSize := len(daroots), len(daroots)/2
	roots := make([]cid.Cid, size)
	for i := 0; i < size; i++ {
		roots[i] = plugin.MustCidFromNamespacedSha256(daroots[i])
	}

	qs := make([]*quadrant, 4) // there are constantly four quadrants
	for i := range qs {
		// convert quadrant index to coordinate
		x, y := i%2, 0
		if i > 1 {
			y = 1
		}

		qs[i] = &quadrant{
			roots: roots[origSize*y : origSize*(y+1)],
			x:     x,
			y:     y,
		}
	}
	// shuffling helps data to be equally distributed around the network
	rand.Shuffle(len(qs), func(i, j int) { qs[i], qs[j] = qs[j], qs[i] })

	ses := r.newSession(ctx, dah)
	for _, q := range qs {
		// TODO: ErrByzantineRow/Col
		eds, err := ses.retrieve(ctx, q)
		if err == nil {
			return eds, nil
		}
		// retry quadrants until we have something or error
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
	defer func() {
		// all shares which were requested or repaired are written to disk with the commit
		// we store *all*, so they are served to the network, including incorrectly committed data(BEFP case),
		// so that network can check BEFP
		if err := rs.adder.Commit(); err != nil {
			log.Errorw("committing DAG", "err", err)
		}
	}()
	// request and fill quadrant's into
	// we don't care about the errors here, just need to request as much data as we can to be able to reconstruct below
	rs.request(ctx, q)

	// try repair
	err := rsmt2d.RepairExtendedDataSquare(rs.dah.RowsRoots, rs.dah.ColumnRoots, rs.square, rs.codec, rs.treeFn)
	if err != nil {
		log.Errorw("not enough shares sampled to recover, retrying...", "err", err)
		return nil, err
	}

	return rsmt2d.ImportExtendedDataSquare(rs.square, rs.codec, rs.treeFn)
}

type quadrant struct {
	roots []cid.Cid
	x, y  int
}

func (rs *retrieverSession) request(ctx context.Context, q *quadrant) {
	ctx, cancel := context.WithTimeout(ctx, RetrieveQuadrantTimeout)
	defer cancel()

	size := len(q.roots)
	for i, root := range q.roots {
		nd, err := rs.dag.Get(ctx, root)
		if err != nil {
			log.Errorw("getting root", "err", err)
			continue // so that we still try for other roots
		}

		// TODO(@Wondertan): GetLeaves should return everything it was able to request even on error,
		// 	so that we fill as much data as possible
		// get leaves of left or right subtree
		nds, err := GetLeaves(ctx, rs.dag, nd.Links()[q.x].Cid, make([]format.Node, 0, size))
		if err != nil {
			log.Errorw("getting all the leaves", "err", err)
			continue
		}
		// fill leaves into the square
		for j, nd := range nds {
			pos := i*size*2 + size*q.x + size*size*2*q.y + j
			rs.square[pos] = nd.RawData()[1+NamespaceSize:]
		}
	}
}
