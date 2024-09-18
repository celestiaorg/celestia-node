package shwap_test

import (
	"context"
	"fmt"
	"github.com/celestiaorg/celestia-node/share/eds/edstest"
	eds "github.com/celestiaorg/celestia-node/share/new_eds"
	"github.com/celestiaorg/celestia-node/share/sharetest"
	"github.com/celestiaorg/celestia-node/share/shwap"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestRangeNamespaceData(t *testing.T) {
	const (
		odsSize = 8
		edsSize = odsSize * odsSize
	)

	ns := sharetest.RandV0Namespace()
	square, root := edstest.RandEDSWithNamespace(t, ns, odsSize, odsSize)

	nsRowStart := -1
	for i, row := range root.RowRoots {
		if !ns.IsOutsideRange(row, row) {
			nsRowStart = i
			break
		}
	}
	assert.Greater(t, nsRowStart, -1)

	extended := &eds.Rsmt2D{ExtendedDataSquare: square}

	nsData, err := extended.RowNamespaceData(context.Background(), ns, nsRowStart)
	require.NoError(t, err)
	col := nsData.Proof.Start()

	sampleID, err := shwap.NewSampleID(1, nsRowStart, col, edsSize)
	require.NoError(t, err)
	for i := 1; i <= odsSize; i++ {
		t.Run(fmt.Sprintf("range of %d shares", i), func(t *testing.T) {
			dataID, err := shwap.NewRangeNamespaceDataID(sampleID, ns, uint16(i), false, edsSize)
			require.NoError(t, err)

			rngdata, err := extended.RangeNamespaceData(
				context.Background(),
				dataID.RangeNamespace,
				dataID.RowIndex,
				dataID.ShareIndex,
				dataID.Length,
				dataID.ProofsOnly,
			)
			require.NoError(t, err)

			roots, err := extended.AxisRoots(context.Background())
			require.NoError(t, err)

			err = rngdata.Verify(roots, &dataID)
			require.NoError(t, err)

			protoRng := rngdata.RangeNamespaceDataToProto()
			data := shwap.ProtoToRangeNamespaceData(protoRng)
			assert.EqualValues(t, rngdata, data)
		})
	}
}
