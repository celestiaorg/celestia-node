package shwap_test

import (
	"context"
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
	sample, err := extended.Sample(context.Background(), nsRowStart, col)
	require.NoError(t, err)

	nsDataRange, err := eds.NamespaceData(context.Background(), extended, ns)
	require.NoError(t, err)

	data := shwap.NewRangeNamespaceData(nsDataRange, &sample)

	dataID, err := shwap.NewRangeNamespaceDataID(sampleID, ns, odsSize, false, edsSize)
	require.NoError(t, err)

	err = data.Verify(root, &dataID)
	require.NoError(t, err)
}
