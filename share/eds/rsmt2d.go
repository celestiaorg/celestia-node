package eds

import (
	"context"
	"fmt"
	"io"

	"github.com/celestiaorg/celestia-app/v5/pkg/wrapper"
	libshare "github.com/celestiaorg/go-square/v2/share"
	"github.com/celestiaorg/rsmt2d"

	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/celestia-node/share/shwap"
)

var _ AccessorStreamer = (*Rsmt2D)(nil)

// Rsmt2D is a rsmt2d based in-memory implementation of Accessor.
type Rsmt2D struct {
	*rsmt2d.ExtendedDataSquare
}

// Size returns the size of the Extended Data Square.
func (eds *Rsmt2D) Size(context.Context) (int, error) {
	return int(eds.Width()), nil
}

// DataHash returns data hash of the Accessor.
func (eds *Rsmt2D) DataHash(context.Context) (share.DataHash, error) {
	roots, err := share.NewAxisRoots(eds.ExtendedDataSquare)
	if err != nil {
		return share.DataHash{}, fmt.Errorf("while creating data root: %w", err)
	}
	return roots.Hash(), nil
}

// AxisRoots returns AxisRoots of the Accessor.
func (eds *Rsmt2D) AxisRoots(context.Context) (*share.AxisRoots, error) {
	roots, err := share.NewAxisRoots(eds.ExtendedDataSquare)
	if err != nil {
		return nil, fmt.Errorf("while creating axis roots: %w", err)
	}
	return roots, nil
}

// Sample returns share and corresponding proof for row and column indices.
func (eds *Rsmt2D) Sample(
	_ context.Context,
	idx shwap.SampleCoords,
) (shwap.Sample, error) {
	return eds.SampleForProofAxis(idx, rsmt2d.Row)
}

// SampleForProofAxis samples a share from an Extended Data Square based on the provided
// row and column indices and proof axis. It returns a sample with the share and proof.
func (eds *Rsmt2D) SampleForProofAxis(
	idx shwap.SampleCoords,
	proofType rsmt2d.Axis,
) (shwap.Sample, error) {
	axisIdx, shrIdx := relativeIndexes(idx.Row, idx.Col, proofType)
	shares, err := getAxis(eds.ExtendedDataSquare, proofType, axisIdx)
	if err != nil {
		return shwap.Sample{}, err
	}

	tree := wrapper.NewErasuredNamespacedMerkleTree(uint64(eds.Width()/2), uint(axisIdx))
	for _, shr := range shares {
		err := tree.Push(shr.ToBytes())
		if err != nil {
			return shwap.Sample{}, fmt.Errorf("while pushing shares to NMT: %w", err)
		}
	}

	prf, err := tree.ProveRange(shrIdx, shrIdx+1)
	if err != nil {
		return shwap.Sample{}, fmt.Errorf("while proving range share over NMT: %w", err)
	}

	return shwap.Sample{
		Share:     shares[shrIdx],
		Proof:     &prf,
		ProofType: proofType,
	}, nil
}

// AxisHalf returns Shares for the first half of the axis of the given type and index.
func (eds *Rsmt2D) AxisHalf(_ context.Context, axisType rsmt2d.Axis, axisIdx int) (shwap.AxisHalf, error) {
	shares, err := getAxis(eds.ExtendedDataSquare, axisType, axisIdx)
	if err != nil {
		return shwap.AxisHalf{}, fmt.Errorf("while getting axis share: %w", err)
	}
	halfShares := shares[:eds.Width()/2]
	return shwap.AxisHalf{
		Shares:   halfShares,
		IsParity: false,
	}, nil
}

// HalfRow constructs a new shwap.Row from an Extended Data Square based on the specified index and
// side.
func (eds *Rsmt2D) HalfRow(idx int, side shwap.RowSide) (shwap.Row, error) {
	return shwap.RowFromEDS(eds.ExtendedDataSquare, idx, side)
}

// RowNamespaceData returns data for the given namespace and row index.
func (eds *Rsmt2D) RowNamespaceData(
	_ context.Context,
	namespace libshare.Namespace,
	rowIdx int,
) (shwap.RowNamespaceData, error) {
	shares := eds.Row(uint(rowIdx))
	sh, err := libshare.FromBytes(shares)
	if err != nil {
		return shwap.RowNamespaceData{}, fmt.Errorf("while converting shares from bytes: %w", err)
	}
	return shwap.RowNamespaceDataFromShares(sh, namespace, rowIdx)
}

// RangeNamespaceData builds a namespace range from the given indexes.
func (eds *Rsmt2D) RangeNamespaceData(
	ctx context.Context,
	from, to int,
) (shwap.RangeNamespaceData, error) {
	size, err := eds.Size(ctx)
	if err != nil {
		return shwap.RangeNamespaceData{}, err
	}

	// get coords based on the odsSize
	odsSize := size / 2
	fromCoords, err := shwap.SampleCoordsFrom1DIndex(from, odsSize)
	if err != nil {
		return shwap.RangeNamespaceData{}, err
	}
	// `to` is an exclusive index. Getting inclusive coordinates of the last share.
	toCoords, err := shwap.SampleCoordsFrom1DIndex(to-1, odsSize)
	if err != nil {
		return shwap.RangeNamespaceData{}, err
	}

	rawShares := make([][]libshare.Share, toCoords.Row-fromCoords.Row+1)
	for row, idx := fromCoords.Row, 0; row <= toCoords.Row; row++ {
		rawShare := eds.Row(uint(row))
		sh, err := libshare.FromBytes(rawShare)
		if err != nil {
			return shwap.RangeNamespaceData{}, err
		}
		rawShares[idx] = sh
		idx++
	}
	return shwap.RangeNamespaceDataFromShares(rawShares, fromCoords, toCoords)
}

// Shares returns data (ODS) shares extracted from the EDS. It returns new copy of the shares each
// time.
func (eds *Rsmt2D) Shares(_ context.Context) ([]libshare.Share, error) {
	return libshare.FromBytes(eds.FlattenedODS())
}

func (eds *Rsmt2D) Close() error {
	return nil
}

// Reader returns binary reader for the file.
func (eds *Rsmt2D) Reader() (io.Reader, error) {
	getShare := func(rowIdx, colIdx int) (libshare.Share, error) {
		rawShare := eds.GetCell(uint(rowIdx), uint(colIdx))

		sh, err := libshare.NewShare(rawShare)
		if err != nil {
			return libshare.Share{}, fmt.Errorf("while creating share: %w", err)
		}
		return *sh, nil
	}
	odsSize := int(eds.Width() / 2)
	reader := NewShareReader(odsSize, getShare)
	return reader, nil
}

// Rsmt2DFromShares constructs an Extended Data Square from shares.
func Rsmt2DFromShares(shares []libshare.Share, odsSize int) (*Rsmt2D, error) {
	treeFn := wrapper.NewConstructor(uint64(odsSize))
	eds, err := rsmt2d.ComputeExtendedDataSquare(libshare.ToBytes(shares), share.DefaultRSMT2DCodec(), treeFn)
	if err != nil {
		return &Rsmt2D{}, fmt.Errorf("computing extended data square: %w", err)
	}

	return &Rsmt2D{eds}, nil
}

func getAxis(eds *rsmt2d.ExtendedDataSquare, axisType rsmt2d.Axis, axisIdx int) ([]libshare.Share, error) {
	switch axisType {
	case rsmt2d.Row:
		return libshare.FromBytes(eds.Row(uint(axisIdx)))
	case rsmt2d.Col:
		return libshare.FromBytes(eds.Col(uint(axisIdx)))
	default:
		panic("unknown axis")
	}
}

func relativeIndexes(rowIdx, colIdx int, axisType rsmt2d.Axis) (axisIdx, shrIdx int) {
	switch axisType {
	case rsmt2d.Row:
		return rowIdx, colIdx
	case rsmt2d.Col:
		return colIdx, rowIdx
	default:
		panic(fmt.Sprintf("invalid proof type: %d", axisType))
	}
}
