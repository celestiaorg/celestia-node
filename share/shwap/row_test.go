package shwap

import (
	shwappb "github.com/celestiaorg/celestia-node/share/shwap/pb"
	"github.com/celestiaorg/rsmt2d"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/celestia-node/share/testing/edstest"
)

func TestRow(t *testing.T) {
	square := edstest.RandEDS(t, 8)
	root, err := share.NewRoot(square)
	require.NoError(t, err)

	row, err := newRowFromEDS(square, 1, 1)
	require.NoError(t, err)

	err = row.Verify(root)
	require.NoError(t, err)
	data, err := row.ToProto().Marshal()
	require.NoError(t, err)

	blk, err := row.IPLDBlock()
	require.NoError(t, err)
	assert.EqualValues(t, blk.Cid(), row.Cid())

	var rowpb shwappb.RowBlock
	err = rowpb.Unmarshal(data)
	require.NoError(t, err)

	rowOut, err := RowFromProto(&rowpb)
	require.NoError(t, err)
	assert.EqualValues(t, row, rowOut)

	err = rowOut.Verify(root)
	require.NoError(t, err)
}

func newRowFromEDS(square *rsmt2d.ExtendedDataSquare, height uint64, idx int) (*Row, error) {
	root, err := share.NewRoot(square)
	if err != nil {
		return nil, err
	}

	id, err := NewRowID(height, uint16(idx), root)
	if err != nil {
		return nil, err
	}

	shares := share.NewRowFromEDS(square, idx, share.Right)
	return &Row{
		RowID:     id,
		RowShares: shares,
	}, nil
}
