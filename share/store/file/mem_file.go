package file

import (
	"context"
	"io"

	"github.com/celestiaorg/celestia-app/pkg/da"
	"github.com/celestiaorg/celestia-app/pkg/wrapper"
	"github.com/celestiaorg/nmt"
	"github.com/celestiaorg/rsmt2d"

	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/celestia-node/share/ipld"
)

var _ EdsFile = (*MemFile)(nil)

type MemFile struct {
	Eds *rsmt2d.ExtendedDataSquare
}

func (f *MemFile) Close() error {
	return nil
}

func (f *MemFile) Reader() (io.Reader, error) {
	return f.readOds().Reader()
}

func (f *MemFile) readOds() square {
	odsLn := int(f.Eds.Width() / 2)
	s := make(square, odsLn)
	for y := 0; y < odsLn; y++ {
		s[y] = make([]share.Share, odsLn)
		for x := 0; x < odsLn; x++ {
			s[y][x] = f.Eds.GetCell(uint(y), uint(x))
		}
	}
	return s
}

func (f *MemFile) DataHash() share.DataHash {
	dah, _ := da.NewDataAvailabilityHeader(f.Eds)
	return dah.Hash()
}

func (f *MemFile) Size() int {
	return int(f.Eds.Width())
}

func (f *MemFile) Share(
	_ context.Context,
	x, y int,
) (*share.ShareWithProof, error) {
	axisType := rsmt2d.Row
	axisIdx, shrIdx := y, x

	shares := getAxis(f.Eds, axisType, axisIdx)
	tree := wrapper.NewErasuredNamespacedMerkleTree(uint64(f.Size()/2), uint(axisIdx))
	for _, shr := range shares {
		err := tree.Push(shr)
		if err != nil {
			return nil, err
		}
	}

	proof, err := tree.ProveRange(shrIdx, shrIdx+1)
	if err != nil {
		return nil, err
	}

	return &share.ShareWithProof{
		Share: shares[shrIdx],
		Proof: &proof,
		Axis:  axisType,
	}, nil
}

func (f *MemFile) AxisHalf(_ context.Context, axisType rsmt2d.Axis, axisIdx int) ([]share.Share, error) {
	return getAxis(f.Eds, axisType, axisIdx)[:f.Size()/2], nil
}

func (f *MemFile) Data(_ context.Context, namespace share.Namespace, rowIdx int) (share.NamespacedRow, error) {
	shares := getAxis(f.Eds, rsmt2d.Row, rowIdx)
	return ndDataFromShares(shares, namespace, rowIdx)
}

func (f *MemFile) EDS(_ context.Context) (*rsmt2d.ExtendedDataSquare, error) {
	return f.Eds, nil
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

func ndDataFromShares(shares []share.Share, namespace share.Namespace, rowIdx int) (share.NamespacedRow, error) {
	bserv := ipld.NewMemBlockservice()
	batchAdder := ipld.NewNmtNodeAdder(context.TODO(), bserv, ipld.MaxSizeBatchOption(len(shares)))
	tree := wrapper.NewErasuredNamespacedMerkleTree(uint64(len(shares)/2), uint(rowIdx),
		nmt.NodeVisitor(batchAdder.Visit))
	for _, shr := range shares {
		err := tree.Push(shr)
		if err != nil {
			return share.NamespacedRow{}, err
		}
	}

	root, err := tree.Root()
	if err != nil {
		return share.NamespacedRow{}, err
	}

	err = batchAdder.Commit()
	if err != nil {
		return share.NamespacedRow{}, err
	}

	row, proof, err := ipld.GetSharesByNamespace(context.TODO(), bserv, root, namespace, len(shares))
	if err != nil {
		return share.NamespacedRow{}, err
	}
	return share.NamespacedRow{
		Shares: row,
		Proof:  proof,
	}, nil
}
