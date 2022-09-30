package share

import (
	"context"
	"fmt"
	"io"
	"math"

	"github.com/tendermint/tendermint/pkg/consts"

	"github.com/ipfs/go-blockservice"
	"github.com/ipfs/go-cid"
	ds "github.com/ipfs/go-datastore"
	dssync "github.com/ipfs/go-datastore/sync"
	blockstore "github.com/ipfs/go-ipfs-blockstore"
	format "github.com/ipfs/go-ipld-format"
	"github.com/ipld/go-car"
	"github.com/ipld/go-car/util"
	"github.com/tendermint/tendermint/pkg/wrapper"

	"github.com/celestiaorg/celestia-node/ipld"
	"github.com/celestiaorg/celestia-node/ipld/plugin"

	"github.com/celestiaorg/nmt"
	"github.com/celestiaorg/rsmt2d"
)

// writingSession contains the components needed to write an EDS to a CARv1 file with our custom node order.
type writingSession struct {
	eds *rsmt2d.ExtendedDataSquare
	// store is an in-memory blockstore, used to cache the inner nodes (proofs) while we walk the nmt tree.
	store blockstore.Blockstore
	w     io.Writer
}

// WriteEDS writes the entire EDS into the given io.Writer as CARv1 file.
// This includes all shares in quadrant order, followed by all inner nodes of the NMT tree.
// Order: Carv1Header - Q1 - Q2 - Q3 - Q4 - InnerNodes
func WriteEDS(ctx context.Context, eds *rsmt2d.ExtendedDataSquare, w io.Writer) error {
	// 1. Reimport EDS. This is needed to traverse the NMT tree and cache the inner nodes (proofs)
	writer, err := initializeWriter(ctx, eds, w)
	if err != nil {
                return fmt.Errorf("share: failure creating eds writer: %w", err)
	}

	// 2. Creates and writes Carv1Header
	//    - Roots are the eds Row + Col roots
	err = writer.writeHeader()
	if err != nil {
                return fmt.Errorf("share: failure writing carv1 header: %w", err)
	}

	// 3. Iterates over shares in quadrant order vis eds.GetCell
	err = writer.writeShares()
	if err != nil {
                return fmt.Errorf("share: failure writing shares: %w", err)
	}

	// 4. Iterates over in-memory Blockstore and writes proofs to the CAR
	err = writer.writeProofs(ctx)
	if err != nil {
                return fmt.Errorf("share: failure writing proofs: %w", err)
	}

	return nil
}

// initializeWriter reimports the EDS into an in-memory blockstore in order to cache the proofs.
func initializeWriter(ctx context.Context, eds *rsmt2d.ExtendedDataSquare, w io.Writer) (*writingSession, error) {
	// we use an in-memory blockstore and an offline exchange
	store := blockstore.NewBlockstore(dssync.MutexWrap(ds.NewMapDatastore()))
	bs := blockservice.New(store, nil)
	// shares are extracted from the eds so that we can reimport them to traverse
	shares := ipld.ExtractEDS(eds)
	if len(shares) == 0 {
		return nil, fmt.Errorf("ipld: importing empty data")
	}
	// todo: add correct batch size here
	squareSize := int(math.Sqrt(float64(len(shares))))
	batchAdder := ipld.NewNmtNodeAdder(ctx, bs, format.MaxSizeBatchOption(squareSize/2))
	// this adder ignores leaves, so that they are not added to the store we iterate through in writeProofs
	tree := wrapper.NewErasuredNamespacedMerkleTree(uint64(squareSize/2), nmt.NodeVisitor(batchAdder.VisitInnerNodes))
	eds, err := rsmt2d.ImportExtendedDataSquare(shares, ipld.DefaultRSMT2DCodec(), tree.Constructor)
	if err != nil {
		return nil, fmt.Errorf("failure to recompute the extended data square: %w", err)
	}
	// compute roots
	eds.RowRoots()
	// commit the batch to DAG
	err = batchAdder.Commit()
	if err != nil {
		return nil, fmt.Errorf("failure to commit the inner nodes to the dag: %w", err)
	}

	return &writingSession{
		eds:   eds,
		store: store,
		w:     w,
	}, nil
}

// writeHeader creates a CarV1 header using the EDS's Row and Column roots as the list of DAG roots.
func (w *writingSession) writeHeader() error {
	rootCids, err := rootsToCids(w.eds)
	if err != nil {
		return fmt.Errorf("failure getting root cids: %w", err)
	}

	err = car.WriteHeader(&car.CarHeader{
		Roots:   rootCids,
		Version: 1,
	}, w.w)
	if err != nil {
		return fmt.Errorf("failure writing carv1 header: %w", err)
	}
	return nil
}

// writeShares reorders the shares to quadrant order and writes them to the CARv1 file.
func (w *writingSession) writeShares() error {
	shares := quadrantOrder(w.eds)
	for _, share := range shares {
		// TODO: Okay. this is really weird and I don't understand:
		// We need to cut off the first byte like we do for inner nodes, but this share doesn't even have the prefix...
		// So what is going on? If we don't do this, the cid doesn't match on read.
		cid, err := plugin.CidFromNamespacedSha256(nmt.Sha256Namespace8FlaggedLeaf(share[1:]))
		if err != nil {
			return fmt.Errorf("failure to get cid from share: %w", err)
		}
		err = util.LdWrite(w.w, cid.Bytes(), share)
		if err != nil {
			return fmt.Errorf("failure to write share: %w", err)
		}
	}
	return nil
}

// writeProofs iterates over the in-memory blockstore's keys and writes all inner nodes to the CARv1 file.
func (w *writingSession) writeProofs(ctx context.Context) error {
	// we only stored proofs to the store, so we can just iterate over them here without getting any leaves
	proofs, err := w.store.AllKeysChan(ctx)
	if err != nil {
		return fmt.Errorf("failure to get all keys from the blockstore: %w", err)
	}
	for proofCid := range proofs {
		node, err := w.store.Get(ctx, proofCid)
		if err != nil {
			return fmt.Errorf("failure to get proof from the blockstore: %w", err)
		}
		// TODO: Learn why this doesn't match proofCid or node.Cid()
		cid, err := plugin.CidFromNamespacedSha256(nmt.Sha256Namespace8FlaggedInner(node.RawData()[1:]))
		if err != nil {
			return fmt.Errorf("failure to get cid: %w", err)
		}
		err = util.LdWrite(w.w, cid.Bytes(), node.RawData())
		if err != nil {
			return fmt.Errorf("failure to write proof to the car: %w", err)
		}
	}
	return nil
}

// quadrantOrder reorders the shares in the EDS to quadrant row-by-row order, adding the wrapped namespace
func quadrantOrder(eds *rsmt2d.ExtendedDataSquare) [][]byte {
	size := eds.Width() * eds.Width()
	shares := make([][]byte, size)

	quadrantWidth := int(eds.Width() / 2)
	quadrantSize := quadrantWidth * quadrantWidth
	for i := 0; i < quadrantWidth; i++ {
		for j := 0; j < quadrantWidth; j++ {
			cells := getQuadrantCells(eds, uint(i), uint(j))
			innerOffset := i*quadrantWidth + j
			for quadrant := 0; quadrant < 4; quadrant++ {
				shares[(quadrant*quadrantSize)+innerOffset] = prependNamespace(quadrant, cells[quadrant])
			}
		}
	}
	return shares
}

// getQuadrantCells returns the cell of each EDS quadrant with the passed inner-quadrant coordinates
func getQuadrantCells(eds *rsmt2d.ExtendedDataSquare, i, j uint) [][]byte {
	cells := make([][]byte, 4)
	quadrantWidth := eds.Width() / 2
	cells[0] = eds.GetCell(i, j)
	cells[1] = eds.GetCell(i, j+quadrantWidth)
	cells[2] = eds.GetCell(i+quadrantWidth, j)
	cells[3] = eds.GetCell(i+quadrantWidth, j+quadrantWidth)
	return cells
}

// prependNamespace adds the namespace to the passed share if in the first quadrant,
// otherwise it adds the ParitySharesNamespace to the beginning.
func prependNamespace(quadrant int, share []byte) []byte {
	switch quadrant {
	case 0:
		return append(share[:8], share...)
	case 1, 2, 3:
		return append(consts.ParitySharesNamespaceID, share...)
	default:
		panic("invalid quadrant")
	}
}

// rootsToCids converts the EDS's Row and Column roots to CIDs.
func rootsToCids(eds *rsmt2d.ExtendedDataSquare) ([]cid.Cid, error) {
	var err error
	roots := append(eds.RowRoots(), eds.ColRoots()...)
	rootCids := make([]cid.Cid, len(roots))
	for i, r := range roots {
		rootCids[i], err = plugin.CidFromNamespacedSha256(r)
		if err != nil {
			return nil, fmt.Errorf("failure to get cid from root: %w", err)
		}
	}
	return rootCids, nil
}
