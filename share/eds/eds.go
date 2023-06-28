package eds

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"math"

	"github.com/ipfs/go-blockservice"
	"github.com/ipfs/go-cid"
	ds "github.com/ipfs/go-datastore"
	dssync "github.com/ipfs/go-datastore/sync"
	bstore "github.com/ipfs/go-ipfs-blockstore"
	format "github.com/ipfs/go-ipld-format"
	"github.com/ipld/go-car"
	"github.com/ipld/go-car/util"
	"github.com/minio/sha256-simd"

	"github.com/celestiaorg/celestia-app/pkg/da"
	"github.com/celestiaorg/celestia-app/pkg/wrapper"
	"github.com/celestiaorg/nmt"
	"github.com/celestiaorg/rsmt2d"

	"github.com/celestiaorg/celestia-node/libs/utils"
	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/celestia-node/share/ipld"
)

var ErrEmptySquare = errors.New("share: importing empty data")

// writingSession contains the components needed to write an EDS to a CARv1 file with our custom
// node order.
type writingSession struct {
	eds    *rsmt2d.ExtendedDataSquare
	store  bstore.Blockstore // caches inner nodes (proofs) while we walk the nmt tree.
	hasher *nmt.NmtHasher
	w      io.Writer
}

// WriteEDS writes the entire EDS into the given io.Writer as CARv1 file.
// This includes all shares in quadrant order, followed by all inner nodes of the NMT tree.
// Order: [ Carv1Header | Q1 |  Q2 | Q3 | Q4 | inner nodes ]
// For more information about the header: https://ipld.io/specs/transport/car/carv1/#header
func WriteEDS(ctx context.Context, eds *rsmt2d.ExtendedDataSquare, w io.Writer) (err error) {
	ctx, span := tracer.Start(ctx, "write-eds")
	defer func() {
		utils.SetStatusAndEnd(span, err)
	}()

	// 1. Reimport EDS. This is needed to traverse the NMT tree and cache the inner nodes (proofs)
	writer, err := initializeWriter(ctx, eds, w)
	if err != nil {
		return fmt.Errorf("share: creating eds writer: %w", err)
	}

	// 2. Creates and writes Carv1Header
	//    - Roots are the eds Row + Col roots
	err = writer.writeHeader()
	if err != nil {
		return fmt.Errorf("share: writing carv1 header: %w", err)
	}

	// 3. Iterates over shares in quadrant order via eds.GetCell
	err = writer.writeQuadrants()
	if err != nil {
		return fmt.Errorf("share: writing shares: %w", err)
	}

	// 4. Iterates over in-memory blockstore and writes proofs to the CAR
	err = writer.writeProofs(ctx)
	if err != nil {
		return fmt.Errorf("share: writing proofs: %w", err)
	}
	return nil
}

// initializeWriter reimports the EDS into an in-memory blockstore in order to cache the proofs.
func initializeWriter(ctx context.Context, eds *rsmt2d.ExtendedDataSquare, w io.Writer) (*writingSession, error) {
	// we use an in-memory blockstore and an offline exchange
	store := bstore.NewBlockstore(dssync.MutexWrap(ds.NewMapDatastore()))
	bs := blockservice.New(store, nil)
	// shares are extracted from the eds so that we can reimport them to traverse
	shares := share.ExtractEDS(eds)
	shareCount := len(shares)
	if shareCount == 0 {
		return nil, ErrEmptySquare
	}
	odsWidth := int(math.Sqrt(float64(shareCount)) / 2)
	// (shareCount*2) - (odsWidth*4) is the amount of inner nodes visited
	batchAdder := ipld.NewNmtNodeAdder(ctx, bs, format.MaxSizeBatchOption(innerNodeBatchSize(shareCount, odsWidth)))
	// this adder ignores leaves, so that they are not added to the store we iterate through in
	// writeProofs
	eds, err := rsmt2d.ImportExtendedDataSquare(
		shares,
		share.DefaultRSMT2DCodec(),
		wrapper.NewConstructor(uint64(odsWidth),
			nmt.NodeVisitor(batchAdder.VisitInnerNodes)),
	)
	if err != nil {
		return nil, fmt.Errorf("recomputing data square: %w", err)
	}
	// compute roots
	eds.RowRoots()
	// commit the batch to DAG
	err = batchAdder.Commit()
	if err != nil {
		return nil, fmt.Errorf("committing inner nodes to the dag: %w", err)
	}

	return &writingSession{
		eds:    eds,
		store:  store,
		hasher: nmt.NewNmtHasher(sha256.New(), share.NamespaceSize, ipld.NMTIgnoreMaxNamespace),
		w:      w,
	}, nil
}

// writeHeader creates a CarV1 header using the EDS's Row and Column roots as the list of DAG roots.
func (w *writingSession) writeHeader() error {
	rootCids, err := rootsToCids(w.eds)
	if err != nil {
		return fmt.Errorf("getting root cids: %w", err)
	}

	return car.WriteHeader(&car.CarHeader{
		Roots:   rootCids,
		Version: 1,
	}, w.w)
}

// writeQuadrants reorders the shares to quadrant order and writes them to the CARv1 file.
func (w *writingSession) writeQuadrants() error {
	shares := quadrantOrder(w.eds)
	for _, share := range shares {
		leaf, err := w.hasher.HashLeaf(share)
		if err != nil {
			return fmt.Errorf("hashing share: %w", err)
		}
		cid, err := ipld.CidFromNamespacedSha256(leaf)
		if err != nil {
			return fmt.Errorf("getting cid from share: %w", err)
		}
		err = util.LdWrite(w.w, cid.Bytes(), share)
		if err != nil {
			return fmt.Errorf("writing share to file: %w", err)
		}
	}
	return nil
}

// writeProofs iterates over the in-memory blockstore's keys and writes all inner nodes to the
// CARv1 file.
func (w *writingSession) writeProofs(ctx context.Context) error {
	// we only stored proofs to the store, so we can just iterate over them here without getting any
	// leaves
	proofs, err := w.store.AllKeysChan(ctx)
	if err != nil {
		return fmt.Errorf("getting all keys from the blockstore: %w", err)
	}
	for proofCid := range proofs {
		block, err := w.store.Get(ctx, proofCid)
		if err != nil {
			return fmt.Errorf("getting proof from the blockstore: %w", err)
		}

		node := block.RawData()
		left, right := node[:ipld.NmtHashSize], node[ipld.NmtHashSize:]
		hash, err := w.hasher.HashNode(left, right)
		if err != nil {
			return fmt.Errorf("hashing node: %w", err)
		}
		cid, err := ipld.CidFromNamespacedSha256(hash)
		if err != nil {
			return fmt.Errorf("getting cid: %w", err)
		}
		err = util.LdWrite(w.w, cid.Bytes(), node)
		if err != nil {
			return fmt.Errorf("writing proof to the car: %w", err)
		}
	}
	return nil
}

// quadrantOrder reorders the shares in the EDS to quadrant row-by-row order, prepending the
// respective namespace to the shares.
// e.g. [ Q1 R1 | Q1 R2 | Q1 R3 | Q1 R4 | Q2 R1 | Q2 R2 .... ]
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
func prependNamespace(quadrant int, shr share.Share) []byte {
	namespacedShare := make([]byte, 0, share.NamespaceSize+share.Size)
	switch quadrant {
	case 0:
		return append(append(namespacedShare, share.GetNamespace(shr)...), shr...)
	case 1, 2, 3:
		return append(append(namespacedShare, share.ParitySharesNamespace...), shr...)
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
		rootCids[i], err = ipld.CidFromNamespacedSha256(r)
		if err != nil {
			return nil, fmt.Errorf("getting cid from root: %w", err)
		}
	}
	return rootCids, nil
}

// ReadEDS reads the first EDS quadrant (1/4) from an io.Reader CAR file.
// Only the first quadrant will be read, which represents the original data.
// The returned EDS is guaranteed to be full and valid against the DataRoot, otherwise ReadEDS
// errors.
func ReadEDS(ctx context.Context, r io.Reader, root share.DataHash) (eds *rsmt2d.ExtendedDataSquare, err error) {
	_, span := tracer.Start(ctx, "read-eds")
	defer func() {
		utils.SetStatusAndEnd(span, err)
	}()

	carReader, err := car.NewCarReader(r)
	if err != nil {
		return nil, fmt.Errorf("share: reading car file: %w", err)
	}

	// car header includes both row and col roots in header
	odsWidth := len(carReader.Header.Roots) / 4
	odsSquareSize := odsWidth * odsWidth
	shares := make([][]byte, odsSquareSize)
	// the first quadrant is stored directly after the header,
	// so we can just read the first odsSquareSize blocks
	for i := 0; i < odsSquareSize; i++ {
		block, err := carReader.Next()
		if err != nil {
			return nil, fmt.Errorf("share: reading next car entry: %w", err)
		}
		// the stored first quadrant shares are wrapped with the namespace twice.
		// we cut it off here, because it is added again while importing to the tree below
		shares[i] = share.GetData(block.RawData())
	}

	eds, err = rsmt2d.ComputeExtendedDataSquare(
		shares,
		share.DefaultRSMT2DCodec(),
		wrapper.NewConstructor(uint64(odsWidth)),
	)
	if err != nil {
		return nil, fmt.Errorf("share: computing eds: %w", err)
	}

	newDah := da.NewDataAvailabilityHeader(eds)
	if !bytes.Equal(newDah.Hash(), root) {
		return nil, fmt.Errorf(
			"share: content integrity mismatch: imported root %s doesn't match expected root %s",
			newDah.Hash(),
			root,
		)
	}
	return eds, nil
}

// innerNodeBatchSize calculates the total number of inner nodes in an EDS,
// to be flushed to the dagstore in a single write.
func innerNodeBatchSize(shareCount int, odsWidth int) int {
	return (shareCount * 2) - (odsWidth * 4)
}
