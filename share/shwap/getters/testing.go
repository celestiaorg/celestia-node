package getters

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/celestiaorg/celestia-app/v2/pkg/da"
	"github.com/celestiaorg/rsmt2d"

	"github.com/celestiaorg/celestia-node/header"
	"github.com/celestiaorg/celestia-node/header/headertest"
	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/celestia-node/share/eds/edstest"
	"github.com/celestiaorg/celestia-node/share/shwap"
)

// TestGetter provides a testing SingleEDSGetter and the root of the EDS it holds.
func TestGetter(t *testing.T) (shwap.Getter, *header.ExtendedHeader) {
	eds := edstest.RandEDS(t, 8)
	roots, err := share.NewAxisRoots(eds)
	eh := headertest.RandExtendedHeaderWithRoot(t, roots)
	require.NoError(t, err)
	return &SingleEDSGetter{
		EDS: eds,
	}, eh
}

// SingleEDSGetter contains a single EDS where data is retrieved from.
// Its primary use is testing, and GetSharesByNamespace is not supported.
type SingleEDSGetter struct {
	EDS *rsmt2d.ExtendedDataSquare
}

// GetShare gets a share from a kept EDS if exist and if the correct root is given.
func (seg *SingleEDSGetter) GetShare(
	_ context.Context,
	header *header.ExtendedHeader,
	row, col int,
) (share.Share, error) {
	err := seg.checkRoots(header.DAH)
	if err != nil {
		return nil, err
	}
	return seg.EDS.GetCell(uint(row), uint(col)), nil
}

// GetEDS returns a kept EDS if the correct root is given.
func (seg *SingleEDSGetter) GetEDS(
	_ context.Context,
	header *header.ExtendedHeader,
) (*rsmt2d.ExtendedDataSquare, error) {
	err := seg.checkRoots(header.DAH)
	if err != nil {
		return nil, err
	}
	return seg.EDS, nil
}

// GetSharesByNamespace returns NamespacedShares from a kept EDS if the correct root is given.
func (seg *SingleEDSGetter) GetSharesByNamespace(context.Context, *header.ExtendedHeader, share.Namespace,
) (shwap.NamespaceData, error) {
	panic("SingleEDSGetter: GetSharesByNamespace is not implemented")
}

func (seg *SingleEDSGetter) checkRoots(roots *share.AxisRoots) error {
	dah, err := da.NewDataAvailabilityHeader(seg.EDS)
	if err != nil {
		return err
	}
	if !roots.Equals(&dah) {
		return fmt.Errorf("unknown EDS: have %s, asked %s", dah.String(), roots.String())
	}
	return nil
}
