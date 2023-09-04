package car

import (
	"bytes"
	"context"
	"crypto/sha256"
	"fmt"

	"github.com/ipfs/go-cid"
	"github.com/ipld/go-car"

	"github.com/celestiaorg/nmt"

	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/celestia-node/share/eds"
	"github.com/celestiaorg/celestia-node/share/ipld"
)

// ReadSharesByNamespace reads all shares within the given namespace for the given root
// by reading them sequentially from the stored CARv1 file in the eds.Store.
// The shares are read sequentially row by row for each quadrant, and are filtered
// by the given namespace.
func ReadSharesByNamespace(
	ctx context.Context,
	store *eds.Store,
	root *share.Root,
	namespace share.Namespace,
) (share.NamespacedShares, error) {
	rowRootCIDs := make([]cid.Cid, 0, len(root.RowRoots))
	for _, row := range root.RowRoots {
		if !namespace.IsOutsideRange(row, row) {
			rowRootCIDs = append(rowRootCIDs, ipld.MustCidFromNamespacedSha256(row))
		}
	}

	if len(rowRootCIDs) == 0 {
		return nil, nil
	}

	r, err := store.GetCAR(ctx, root.Hash())
	if err != nil {
		return nil, fmt.Errorf("car: failed to retrieve CAR for root %s: %w", root.Hash(), err)
	}

	carReader, err := car.NewCarReader(r)
	if err != nil {
		return nil, fmt.Errorf("car: reading car file: %w", err)
	}

	// shares are ordered by quadrant row-by-row
	// e.g: [ Q1 R1 | Q1 R2 | Q1 R3 | Q1 R4 | Q2 R1 | Q2 R2 .... ]
	// thus we read the car file in quadrants
	odsWidth := len(carReader.Header.Roots) / 4
	odsSquareSize := odsWidth * odsWidth

	shares := make([][]byte, odsSquareSize*2)
	// the first and second quadrant are stored directly after the header,
	// so we can just read the first odsSquareSize*2 blocks
	for i := 0; i < odsSquareSize*2; i++ {
		block, err := carReader.Next()
		if err != nil {
			return nil, fmt.Errorf("share: reading next car entry: %w", err)
		}
		shares[i] = block.RawData()
	}

	// construct eds rows by concatenating rows of the first
	// and second quadrant that are on the same level
	// [ Q1 R1 | Q2 R1 ]
	// [ Q1 R2 | Q2 R2 ]
	// [ Q1 R3 | Q2 R3 ]
	// [ Q1 R4 | Q2 R4 ]
	// this is done to be able to compute the row roots and proofs
	rows := make([][][]byte, odsWidth)
	for i := 0; i < odsWidth; i++ {
		row := make([][]byte, 0)
		row = append(row, shares[i*odsWidth:(i+1)*odsWidth]...)
		row = append(row, shares[odsSquareSize+i*odsWidth:odsSquareSize+(i+1)*odsWidth]...)
		rows[i] = row
	}

	namespacedShares := make(share.NamespacedShares, 0)
	for _, row := range rows {
		rowRoot, proof, err := getRowRootAndProof(row, namespace)
		if err != nil {
			return nil, fmt.Errorf("car: failed to compute row root and proof: %w", err)
		}

		// skip row if it is outside the namespace
		if namespace.IsOutsideRange(rowRoot, rowRoot) {
			continue
		}

		// remove the prepended namespace from the shares
		rowShares := make([][]byte, 0)
		for _, shr := range row {
			if bytes.Equal(share.GetNamespace(shr), namespace.ToNMT()) {
				rowShares = append(rowShares, share.GetData(shr))
			}
		}

		namespacedShares = append(namespacedShares, share.NamespacedRow{
			Shares: rowShares,
			Proof:  proof,
		})
	}

	return namespacedShares, nil
}

// getRowRootAndProof computes the nmt root and proof for a given row of shares.
func getRowRootAndProof(row [][]byte, namespace share.Namespace) ([]byte, *nmt.Proof, error) {
	tree := nmt.New(sha256.New(), nmt.NamespaceIDSize(share.NamespaceSize), nmt.IgnoreMaxNamespace(true))

	// we push raw eds shares to the tree, which are wrapped with the namespace twice
	for _, d := range row {
		if err := tree.Push(d); err != nil {
			return nil, nil, fmt.Errorf("car: failed to build nmt tree for namespace proving: %w", err)
		}
	}

	// compute the root
	root, err := tree.Root()
	if err != nil {
		return nil, nil, fmt.Errorf("car: failed to compute nmt tree root for row: %w", err)
	}

	proof, err := tree.ProveNamespace(namespace.ToNMT())
	if err != nil {
		return nil, nil, fmt.Errorf("car: failed to compute namespace proof: %w", err)
	}

	return root, &proof, nil
}
