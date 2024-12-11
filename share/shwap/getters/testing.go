package getters

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/celestiaorg/celestia-app/v3/pkg/da"
	libshare "github.com/celestiaorg/go-square/v2/share"
	"github.com/celestiaorg/rsmt2d"

	"github.com/celestiaorg/celestia-node/header"
	"github.com/celestiaorg/celestia-node/header/headertest"
	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/celestia-node/share/eds"
	"github.com/celestiaorg/celestia-node/share/eds/edstest"
	"github.com/celestiaorg/celestia-node/share/shwap"
)

// TestGetter provides a testing SingleEDSGetter and the root of the EDS it holds.
func TestGetter(t *testing.T) (shwap.Getter, *header.ExtendedHeader) {
	square := edstest.RandEDS(t, 8)
	roots, err := share.NewAxisRoots(square)
	eh := headertest.RandExtendedHeaderWithRoot(t, roots)
	require.NoError(t, err)
	return &SingleEDSGetter{
		EDS: eds.Rsmt2D{ExtendedDataSquare: square},
	}, eh
}

// SingleEDSGetter contains a single EDS where data is retrieved from.
// Its primary use is testing, and GetNamespaceData is not supported.
type SingleEDSGetter struct {
	EDS eds.Rsmt2D
}

// GetSamples get samples from a kept EDS if exist and if the correct root is given.
func (seg *SingleEDSGetter) GetSamples(ctx context.Context, hdr *header.ExtendedHeader,
	indices []shwap.SampleCoords,
) ([]shwap.Sample, error) {
	err := seg.checkRoots(hdr.DAH)
	if err != nil {
		return nil, err
	}

	smpls := make([]shwap.Sample, len(indices))
	for i, idx := range indices {
		smpl, err := seg.EDS.Sample(ctx, idx)
		if err != nil {
			return nil, err
		}

		smpls[i] = smpl
	}

	return smpls, nil
}

func (seg *SingleEDSGetter) GetRow(
	ctx context.Context,
	header *header.ExtendedHeader,
	rowIdx int,
) (shwap.Row, error) {
	err := seg.checkRoots(header.DAH)
	if err != nil {
		return shwap.Row{}, err
	}

	axisHalf, err := seg.EDS.AxisHalf(ctx, rsmt2d.Row, rowIdx)
	if err != nil {
		return shwap.Row{}, err
	}
	return axisHalf.ToRow(), nil
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
	return seg.EDS.ExtendedDataSquare, nil
}

// GetNamespaceData returns NamespacedShares from a kept EDS if the correct root is given.
func (seg *SingleEDSGetter) GetNamespaceData(context.Context, *header.ExtendedHeader, libshare.Namespace,
) (shwap.NamespaceData, error) {
	panic("SingleEDSGetter: GetNamespaceData is not implemented")
}

func (seg *SingleEDSGetter) checkRoots(roots *share.AxisRoots) error {
	dah, err := da.NewDataAvailabilityHeader(seg.EDS.ExtendedDataSquare)
	if err != nil {
		return err
	}
	if !roots.Equals(&dah) {
		return fmt.Errorf("unknown EDS: have %s, asked %s", dah.String(), roots.String())
	}
	return nil
}
