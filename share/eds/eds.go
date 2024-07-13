package eds

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"math"

	"github.com/ipfs/go-cid"
	"github.com/ipld/go-car"
	"github.com/ipld/go-car/util"
	"github.com/tendermint/tendermint/types"

	pkgproof "github.com/celestiaorg/celestia-app/pkg/proof"
	"github.com/celestiaorg/celestia-app/pkg/shares"
	"github.com/celestiaorg/celestia-app/pkg/wrapper"
	"github.com/celestiaorg/nmt"
	"github.com/celestiaorg/rsmt2d"

	"github.com/celestiaorg/celestia-node/libs/utils"
	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/celestia-node/share/ipld"
)

var ErrEmptySquare = errors.New("share: importing empty data")

// WriteEDS writes the entire EDS into the given io.Writer as CARv1 file.
// This includes all shares in quadrant order, followed by all inner nodes of the NMT tree.
// Order: [ Carv1Header | Q1 |  Q2 | Q3 | Q4 | inner nodes ]
// For more information about the header: https://ipld.io/specs/transport/car/carv1/#header
func WriteEDS(ctx context.Context, eds *rsmt2d.ExtendedDataSquare, w io.Writer) (err error) {
	ctx, span := tracer.Start(ctx, "write-eds")
	defer func() {
		utils.SetStatusAndEnd(span, err)
	}()

	// Creates and writes Carv1Header. Roots are the eds Row + Col roots
	err = writeHeader(eds, w)
	if err != nil {
		return fmt.Errorf("share: writing carv1 header: %w", err)
	}
	// Iterates over shares in quadrant order via eds.GetCell
	err = writeQuadrants(eds, w)
	if err != nil {
		return fmt.Errorf("share: writing shares: %w", err)
	}

	// Iterates over proofs and writes them to the CAR
	err = writeProofs(ctx, eds, w)
	if err != nil {
		return fmt.Errorf("share: writing proofs: %w", err)
	}
	return nil
}

// writeHeader creates a CarV1 header using the EDS's Row and Column roots as the list of DAG roots.
func writeHeader(eds *rsmt2d.ExtendedDataSquare, w io.Writer) error {
	rootCids, err := rootsToCids(eds)
	if err != nil {
		return fmt.Errorf("getting root cids: %w", err)
	}

	return car.WriteHeader(&car.CarHeader{
		Roots:   rootCids,
		Version: 1,
	}, w)
}

// writeQuadrants reorders the shares to quadrant order and writes them to the CARv1 file.
func writeQuadrants(eds *rsmt2d.ExtendedDataSquare, w io.Writer) error {
	hasher := nmt.NewNmtHasher(share.NewSHA256Hasher(), share.NamespaceSize, ipld.NMTIgnoreMaxNamespace)
	shares := quadrantOrder(eds)
	for _, share := range shares {
		leaf, err := hasher.HashLeaf(share)
		if err != nil {
			return fmt.Errorf("hashing share: %w", err)
		}
		cid, err := ipld.CidFromNamespacedSha256(leaf)
		if err != nil {
			return fmt.Errorf("getting cid from share: %w", err)
		}
		err = util.LdWrite(w, cid.Bytes(), share)
		if err != nil {
			return fmt.Errorf("writing share to file: %w", err)
		}
	}
	return nil
}

// writeProofs iterates over the in-memory blockstore's keys and writes all inner nodes to the
// CARv1 file.
func writeProofs(ctx context.Context, eds *rsmt2d.ExtendedDataSquare, w io.Writer) error {
	// check if proofs are collected by ipld.ProofsAdder in previous reconstructions of eds
	proofs, err := getProofs(ctx, eds)
	if err != nil {
		return fmt.Errorf("recomputing proofs: %w", err)
	}

	for id, proof := range proofs {
		err := util.LdWrite(w, id.Bytes(), proof)
		if err != nil {
			return fmt.Errorf("writing proof to the car: %w", err)
		}
	}
	return nil
}

func getProofs(ctx context.Context, eds *rsmt2d.ExtendedDataSquare) (map[cid.Cid][]byte, error) {
	// check if there are proofs collected by ipld.ProofsAdder in previous reconstruction of eds
	if adder := ipld.ProofsAdderFromCtx(ctx); adder != nil {
		defer adder.Purge()
		return adder.Proofs(), nil
	}

	// recompute proofs from eds
	shares := eds.Flattened()
	shareCount := len(shares)
	if shareCount == 0 {
		return nil, ErrEmptySquare
	}
	odsWidth := int(math.Sqrt(float64(shareCount)) / 2)

	// this adder ignores leaves, so that they are not added to the store we iterate through in
	// writeProofs
	adder := ipld.NewProofsAdder(odsWidth * 2)
	defer adder.Purge()

	eds, err := rsmt2d.ImportExtendedDataSquare(
		shares,
		share.DefaultRSMT2DCodec(),
		wrapper.NewConstructor(uint64(odsWidth),
			nmt.NodeVisitor(adder.VisitFn())),
	)
	if err != nil {
		return nil, fmt.Errorf("recomputing data square: %w", err)
	}
	// compute roots
	if _, err = eds.RowRoots(); err != nil {
		return nil, fmt.Errorf("computing row roots: %w", err)
	}

	return adder.Proofs(), nil
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
	rowRoots, err := eds.RowRoots()
	if err != nil {
		return nil, err
	}
	colRoots, err := eds.ColRoots()
	if err != nil {
		return nil, err
	}

	roots := make([][]byte, 0, len(rowRoots)+len(colRoots))
	roots = append(roots, rowRoots...)
	roots = append(roots, colRoots...)
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

	// use proofs adder if provided, to cache collected proofs while recomputing the eds
	var opts []nmt.Option
	visitor := ipld.ProofsAdderFromCtx(ctx).VisitFn()
	if visitor != nil {
		opts = append(opts, nmt.NodeVisitor(visitor))
	}

	eds, err = rsmt2d.ComputeExtendedDataSquare(
		shares,
		share.DefaultRSMT2DCodec(),
		wrapper.NewConstructor(uint64(odsWidth), opts...),
	)
	if err != nil {
		return nil, fmt.Errorf("share: computing eds: %w", err)
	}

	newDah, err := share.NewRoot(eds)
	if err != nil {
		return nil, err
	}
	if !bytes.Equal(newDah.Hash(), root) {
		return nil, fmt.Errorf(
			"share: content integrity mismatch: imported root %s doesn't match expected root %s",
			newDah.Hash(),
			root,
		)
	}
	return eds, nil
}

// ProveShares generates a share proof for a share range.
// The share range, defined by start and end, is end-exclusive.
func ProveShares(eds *rsmt2d.ExtendedDataSquare, start, end int) (*types.ShareProof, error) {
	log.Debugw("proving share range", "start", start, "end", end)
	if start == end {
		return nil, fmt.Errorf("start share cannot be equal to end share")
	}
	if start > end {
		return nil, fmt.Errorf("start share %d cannot be greater than end share %d", start, end)
	}

	odsShares, err := shares.FromBytes(eds.FlattenedODS())
	if err != nil {
		return nil, err
	}
	nID, err := pkgproof.ParseNamespace(odsShares, start, end)
	if err != nil {
		return nil, err
	}
	log.Debugw("generating the share proof", "start", start, "end", end)
	proof, err := pkgproof.NewShareInclusionProofFromEDS(eds, nID, shares.NewRange(start, end))
	if err != nil {
		return nil, err
	}
	return &proof, nil
}
