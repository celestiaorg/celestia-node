package eds

import (
	"context"

	"github.com/celestiaorg/celestia-app/pkg/wrapper"
	"github.com/celestiaorg/rsmt2d"

	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/celestia-node/share/eds/byzantine"
)

type MemFile struct {
	Eds *rsmt2d.ExtendedDataSquare
}

func (f *MemFile) Close() error {
	return nil
}

func (f *MemFile) Size() int {
	return int(f.Eds.Width())
}

func (f *MemFile) ShareWithProof(
	_ context.Context,
	idx int,
	axis rsmt2d.Axis,
	_ []byte,
	// TODO: move ShareWithProof to share pkg
) (*byzantine.ShareWithProof, error) {
	sqrLn := f.Size()
	axsIdx, shrIdx := idx/sqrLn, idx%sqrLn
	if axis == rsmt2d.Col {
		axsIdx, shrIdx = shrIdx, axsIdx
	}

	shrs, err := f.Axis(axsIdx, axis)
	if err != nil {
		return nil, err
	}

	// TODO(@Wondartan): this must access cached NMT on EDS instead of computing a new one
	tree := wrapper.NewErasuredNamespacedMerkleTree(uint64(sqrLn/2), uint(axsIdx))
	for _, shr := range shrs {
		err = tree.Push(shr)
		if err != nil {
			return nil, err
		}
	}

	proof, err := tree.ProveRange(shrIdx, shrIdx+1)
	if err != nil {
		return nil, err
	}

	return &byzantine.ShareWithProof{
		Share: shrs[shrIdx],
		Proof: proof,
	}, nil
}

func (f *MemFile) Axis(idx int, axis rsmt2d.Axis) ([]share.Share, error) {
	return getAxis(idx, axis, f.Eds), nil
}

func (f *MemFile) AxisHalf(idx int, axis rsmt2d.Axis) ([]share.Share, error) {
	return getAxis(idx, axis, f.Eds)[:f.Size()/2], nil
}

func (f *MemFile) EDS() (*rsmt2d.ExtendedDataSquare, error) {
	return f.Eds, nil
}

// TODO(@Wondertan): Should be a method on eds
func getAxis(idx int, axis rsmt2d.Axis, eds *rsmt2d.ExtendedDataSquare) [][]byte {
	switch axis {
	case rsmt2d.Row:
		return eds.Row(uint(idx))
	case rsmt2d.Col:
		return eds.Col(uint(idx))
	default:
		panic("unknown axis")
	}
}
