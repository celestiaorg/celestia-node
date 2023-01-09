package getters

import (
	"context"
	"fmt"

	"github.com/celestiaorg/celestia-app/pkg/da"
	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/nmt/namespace"
	"github.com/celestiaorg/rsmt2d"
)

// SingleEDSGetter contains single EDS and allows getting data only out of it
type SingleEDSGetter struct {
	EDS *rsmt2d.ExtendedDataSquare
}

func (seg *SingleEDSGetter) GetShare(_ context.Context, root *share.Root, row, col int) (share.Share, error) {
	err := seg.checkRoot(root)
	if err != nil {
		return nil, err
	}
	return seg.EDS.GetCell(uint(row), uint(col)), nil
}

func (seg *SingleEDSGetter) GetEDS(_ context.Context, root *share.Root) (*rsmt2d.ExtendedDataSquare, error) {
	err := seg.checkRoot(root)
	if err != nil {
		return nil, err
	}
	return seg.EDS, nil
}

func (seg *SingleEDSGetter) GetSharesByNamespace(
	ctx context.Context,
	root *share.Root,
	id namespace.ID,
) (share.NamespacedShares, error) {
	panic("SingleEDSGetter: GetSharesByNamespace is not implemented")
}

func (seg *SingleEDSGetter) checkRoot(root *share.Root) error {
	dah := da.NewDataAvailabilityHeader(seg.EDS)
	if !root.Equals(&dah) {
		return fmt.Errorf("unknown EDS: have %s, asked %s", dah.String(), root.String())
	}
	return nil
}
