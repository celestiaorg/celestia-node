package eds

import (
	"context"
	"fmt"
	"io"

	"github.com/celestiaorg/celestia-app/pkg/wrapper"
	"github.com/celestiaorg/rsmt2d"

	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/celestia-node/share/shwap"
)

// var _ Accessor = Rsmt2D{}

// Rsmt2D is a rsmt2d based in-memory implementation of Accessor.
type Rsmt2D struct {
	*rsmt2d.ExtendedDataSquare
}

// Size returns the size of the Extended Data Square.
func (eds *Rsmt2D) Size(context.Context) int {
	return int(eds.Width())
}

// Sample returns share and corresponding proof for row and column indices.
func (eds *Rsmt2D) Sample(
	_ context.Context,
	rowIdx, colIdx int,
) (shwap.Sample, error) {
	return eds.SampleForProofAxis(rowIdx, colIdx, rsmt2d.Row)
}

// SampleForProofAxis samples a share from an Extended Data Square based on the provided
// row and column indices and proof axis. It returns a sample with the share and proof.
func (eds *Rsmt2D) SampleForProofAxis(
	rowIdx, colIdx int,
	proofType rsmt2d.Axis,
) (shwap.Sample, error) {
	axisIdx, shrIdx := relativeIndexes(rowIdx, colIdx, proofType)
	shares := getAxis(eds.ExtendedDataSquare, proofType, axisIdx)

	tree := wrapper.NewErasuredNamespacedMerkleTree(uint64(eds.Width()/2), uint(axisIdx))
	for _, shr := range shares {
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
		Share:     shares[shrIdx],
		Proof:     &prf,
		ProofType: proofType,
	}, nil
}

// AxisHalf returns Shares for the first half of the axis of the given type and index.
func (eds *Rsmt2D) AxisHalf(_ context.Context, axisType rsmt2d.Axis, axisIdx int) (AxisHalf, error) {
	shares := getAxis(eds.ExtendedDataSquare, axisType, axisIdx)
	halfShares := shares[:eds.Width()/2]
	return AxisHalf{
		Shares:   halfShares,
		IsParity: false,
	}, nil
}

// HalfRow constructs a new shwap.Row from an Extended Data Square based on the specified index and
// side.
func (eds *Rsmt2D) HalfRow(idx int, side shwap.RowSide) shwap.Row {
	shares := eds.ExtendedDataSquare.Row(uint(idx))
	return shwap.RowFromShares(shares, side)
}

// RowNamespaceData returns data for the given namespace and row index.
func (eds *Rsmt2D) RowNamespaceData(
	_ context.Context,
	namespace share.Namespace,
	rowIdx int,
) (shwap.RowNamespaceData, error) {
	shares := eds.Row(uint(rowIdx))
	return shwap.RowNamespaceDataFromShares(shares, namespace, rowIdx)
}

// Shares returns data (ODS) shares extracted from the EDS. It returns new copy of the shares each
// time.
func (eds *Rsmt2D) Shares(_ context.Context) ([]share.Share, error) {
	return eds.ExtendedDataSquare.FlattenedODS(), nil
}

func (eds *Rsmt2D) Close() error {
	return nil
}

// Reader returns binary reader for the file.
func (eds *Rsmt2D) Reader() (io.Reader, error) {
	edsCopy := newEDSCopy(eds)
	reader := NewBufferedReader(edsCopy)
	return reader, nil
}

// ReadFrom reads shares from the provided reader and constructs an Extended Data Square. Provided
// reader should contain shares in row-major order.
func Rsmt2DFromShares(shares []share.Share, odsSize int) (Rsmt2D, error) {
	treeFn := wrapper.NewConstructor(uint64(odsSize))
	eds, err := rsmt2d.ComputeExtendedDataSquare(shares, share.DefaultRSMT2DCodec(), treeFn)
	if err != nil {
		return Rsmt2D{}, fmt.Errorf("computing extended data square: %w", err)
	}

	return Rsmt2D{eds}, nil
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

type edsCopy struct {
	eds *Rsmt2D
	// current is the amount of Shares stored in square that have been read from reader. When current
	// reaches total, bufferedODSReader will prevent further reads by returning io.EOF
	current, total uint
}

func newEDSCopy(eds *Rsmt2D) *edsCopy {
	return &edsCopy{
		eds:     eds,
		current: 0,
		total:   eds.Width() * eds.Width() / 4,
	}
}

func (w *edsCopy) CopyAtLeast(writer io.Writer, limit int) (int, error) {
	// read Shares to the buffer until it has sufficient data to fill provided container or full square
	// is read
	if w.current >= w.total {
		return 0, io.EOF
	}
	odsLn := w.eds.Width() / 2
	var written int
	for w.current < w.total && written < limit {
		rowIdx, colIdx := w.current/(odsLn), w.current%(odsLn)
		n, err := writer.Write(w.eds.GetCell(rowIdx, colIdx))
		w.current++
		written += n
		if err != nil {
			return written, err
		}
	}
	return written, nil
}
