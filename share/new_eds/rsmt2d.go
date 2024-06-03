package eds

import (
	"context"
	"fmt"

	"github.com/celestiaorg/celestia-app/pkg/wrapper"
	"github.com/celestiaorg/rsmt2d"

	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/celestia-node/share/shwap"
)

var _ Accessor = Rsmt2D{}

// Rsmt2D is a rsmt2d based in-memory implementation of Accessor.
type Rsmt2D struct {
	*rsmt2d.ExtendedDataSquare
}

// Close does nothing.
func (eds Rsmt2D) Close() error {
	return nil
}

// Size returns the size of the Extended Data Square.
func (eds Rsmt2D) Size() int {
	return int(eds.Width())
}

// Sample returns share and corresponding proof for row and column indices.
func (eds Rsmt2D) Sample(
	_ context.Context,
	rowIdx, colIdx int,
) (shwap.Sample, error) {
	return eds.SampleForProofAxis(rowIdx, colIdx, rsmt2d.Row)
}

// SampleForProofAxis samples a share from an Extended Data Square based on the provided
// row and column indices and proof axis. It returns a sample with the share and proof.
func (eds Rsmt2D) SampleForProofAxis(
	rowIdx, colIdx int,
	proofType rsmt2d.Axis,
) (shwap.Sample, error) {
	var axisIdx, shrIdx int
	switch proofType {
	case rsmt2d.Row:
		axisIdx, shrIdx = rowIdx, colIdx
	case rsmt2d.Col:
		axisIdx, shrIdx = colIdx, rowIdx
	default:
		return shwap.Sample{}, fmt.Errorf("invalid proof type: %d", proofType)
	}
	shrs := getAxis(eds.ExtendedDataSquare, proofType, axisIdx)

	tree := wrapper.NewErasuredNamespacedMerkleTree(uint64(eds.Width()/2), uint(axisIdx))
	for _, shr := range shrs {
		err := tree.Push(shr)
		if err != nil {
			return shwap.Sample{}, fmt.Errorf("while pushing shares to NMT: %w", err)
		}
	}

	prf, err := tree.ProveRange(shrIdx, shrIdx+1)
	if err != nil {
		return shwap.Sample{}, fmt.Errorf("while proving range share over NMT: %w", err)
	}

	return shwap.Sample{
		Share:     shrs[shrIdx],
		Proof:     &prf,
		ProofType: proofType,
	}, nil
}

// AxisHalf returns Shares for the first half of the axis of the given type and index.
func (eds Rsmt2D) AxisHalf(_ context.Context, axisType rsmt2d.Axis, axisIdx int) (AxisHalf, error) {
	return AxisHalf{
		Shares:   getAxis(eds.ExtendedDataSquare, axisType, axisIdx)[:eds.Size()/2],
		IsParity: false,
	}, nil
}

// HalfRow constructs a new shwap.Row from an Extended Data Square based on the specified index and
// side.
func (eds Rsmt2D) HalfRow(idx int, side shwap.RowSide) shwap.Row {
	shares := eds.ExtendedDataSquare.Row(uint(idx))
	return shwap.RowFromShares(shares, side)
}

// RowNamespaceData returns data for the given namespace and row index.
func (eds Rsmt2D) RowNamespaceData(
	_ context.Context,
	namespace share.Namespace,
	rowIdx int,
) (shwap.RowNamespaceData, error) {
	shares := eds.Row(uint(rowIdx))
	return shwap.RowNamespaceDataFromShares(shares, namespace, rowIdx)
}

// Flattened returns data shares extracted from the EDS.
func (eds Rsmt2D) Flattened(_ context.Context) ([]share.Share, error) {
	// Flattened returns copy of shares, which is not efficient.
	return eds.ExtendedDataSquare.Flattened(), nil
}

func getAxis(eds *rsmt2d.ExtendedDataSquare, axisType rsmt2d.Axis, axisIdx int) []share.Share {
	switch axisType {
	case rsmt2d.Row:
		return eds.Row(uint(axisIdx))
	case rsmt2d.Col:
		return eds.Col(uint(axisIdx))
	default:
		panic("unknown axis")
	}
}
