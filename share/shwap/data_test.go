package shwap

import (
	"github.com/celestiaorg/rsmt2d"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/celestiaorg/celestia-node/share"
	shwappb "github.com/celestiaorg/celestia-node/share/shwap/pb"
	"github.com/celestiaorg/celestia-node/share/testing/edstest"
	"github.com/celestiaorg/celestia-node/share/testing/sharetest"
)

func TestData(t *testing.T) {
	namespace := sharetest.RandV0Namespace()
	square, root := edstest.RandEDSWithNamespace(t, namespace, 16, 8)

	datas, err := newDataFromEDS(square, 1, namespace, 0)
	require.NoError(t, err)

	data := datas[0]
	// check bitswap encoding
	blk, err := data.IPLDBlock()
	require.NoError(t, err)
	assert.EqualValues(t, blk.Cid(), data.Cid())

	out, err := DataFromBlock(blk)
	require.NoError(t, err)
	assert.EqualValues(t, data, out)

	// check proto encoding
	bin, err := data.ToProto().Marshal()
	require.NoError(t, err)

	var datapb shwappb.DataBlock
	err = datapb.Unmarshal(bin)
	require.NoError(t, err)

	dataOut, err := DataFromProto(&datapb)
	require.NoError(t, err)
	assert.EqualValues(t, data, dataOut)

	err = dataOut.Verify(root)
	require.NoError(t, err)
}

func newDataFromEDS(square *rsmt2d.ExtendedDataSquare, height uint64, namespace share.Namespace) ([]Data, error) {
	root, err := share.NewRoot(square)
	if err != nil {
		return nil, err
	}

	var datas []Data
	for i := 0; i < len(root.RowRoots); i++ {
		rowRoot := root.RowRoots[i]
		if !namespace.IsOutsideRange(rowRoot, rowRoot) {
			shares := square.Row(uint(i))
			nr, err := share.NamespacedRowFromShares(shares, namespace, i)
			if err != nil {
				return nil, err
			}
			id, err := NewDataID(height, uint16(i), namespace, root)
			datas = append(datas, Data{
				DataID:        id,
				NamespacedRow: nr,
			})
		}
	}

	return datas, nil
}
