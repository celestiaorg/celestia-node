package eds

import (
	"github.com/celestiaorg/celestia-app/pkg/wrapper"
	"github.com/celestiaorg/nmt"
	"github.com/celestiaorg/rsmt2d"

	"github.com/celestiaorg/celestia-node/share"
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

func (f *MemFile) ShareWithProof(axisType rsmt2d.Axis, axisIdx, shrIdx int) (share.Share, nmt.Proof, error) {
	sqrLn := f.Size()
	shrs, err := f.Axis(axisType, axisIdx)
	if err != nil {
		return nil, nmt.Proof{}, err
	}

	// TODO(@Wondartan): this must access cached NMT on EDS instead of computing a new one
	tree := wrapper.NewErasuredNamespacedMerkleTree(uint64(sqrLn/2), uint(axisIdx))
	for _, shr := range shrs {
		err = tree.Push(shr)
		if err != nil {
			return nil, nmt.Proof{}, err
		}
	}

	proof, err := tree.ProveRange(shrIdx, shrIdx+1)
	if err != nil {
		return nil, nmt.Proof{}, err
	}

	return shrs[shrIdx], proof, nil
}

func (f *MemFile) Axis(axisType rsmt2d.Axis, axisIdx int) ([]share.Share, error) {
	return getAxis(axisType, axisIdx, f.Eds), nil
}

func (f *MemFile) AxisHalf(axisType rsmt2d.Axis, axisIdx int) ([]share.Share, error) {
	return getAxis(axisType, axisIdx, f.Eds)[:f.Size()/2], nil
}

func (f *MemFile) Data(namespace share.Namespace, axisIdx int) ([]share.Share, nmt.Proof, error) {
	shrs, err := f.Axis(rsmt2d.Row, axisIdx)
	if err != nil {
		return nil, nmt.Proof{}, err
	}

	return NDFromShares(shrs, namespace, axisIdx)
}

func (f *MemFile) EDS() (*rsmt2d.ExtendedDataSquare, error) {
	return f.Eds, nil
}

// TODO(@Wondertan): Should be a method on eds
func getAxis(axisType rsmt2d.Axis, axisIdx int, eds *rsmt2d.ExtendedDataSquare) [][]byte {
	switch axisType {
	case rsmt2d.Row:
		return eds.Row(uint(axisIdx))
	case rsmt2d.Col:
		return eds.Col(uint(axisIdx))
	default:
		panic("unknown axis")
	}
}
