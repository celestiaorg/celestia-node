package fraud

import (
	"context"
	"errors"
	"testing"
	"time"

	mdutils "github.com/ipfs/go-merkledag/test"
	"github.com/stretchr/testify/require"
	"github.com/tendermint/tendermint/pkg/consts"
	"github.com/tendermint/tendermint/pkg/da"
	"github.com/tendermint/tendermint/pkg/wrapper"

	"github.com/celestiaorg/celestia-node/ipld"
	"github.com/celestiaorg/celestia-node/service/header"
	"github.com/celestiaorg/rsmt2d"
)

func TestCreateBadEncodingFraudProofWithCompletedShares(t *testing.T) {
	eds := ipld.RandEDS(t, 16)
	type test struct {
		name   string
		isRow  bool
		roots  [][]byte
		leaves func(uint) [][]byte
		length int
	}
	tests := []test{
		{"rowRoots", true, eds.RowRoots(), eds.Row, len(eds.RowRoots())},
		{"colRoots", false, eds.ColRoots(), eds.Col, len(eds.ColRoots())},
	}
	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			for index := 0; index < tc.length; index++ {
				_, err := CreateBadEncodingProof(
					context.Background(),
					1,
					uint8(index),
					tc.isRow,
					eds,
					tc.roots,
					tc.leaves(uint(index)),
					nil,
				)
				require.NoError(t, err)
			}
		})
	}
}

func TestFraudProofValidationForRow(t *testing.T) {
	dag := mdutils.Mock()
	eds := ipld.RandEDS(t, 2)
	size := eds.Width()

	tree := wrapper.NewErasuredNamespacedMerkleTree(uint64(size / 2))
	_, err := ipld.PutData(context.Background(), ipld.ExtractODSShares(eds), dag)
	require.NoError(t, err)
	shares := flatten(eds)
	copy(shares[3][8:], shares[8][8:])
	attackerEDS, err := rsmt2d.ImportExtendedDataSquare(shares, consts.DefaultCodec(), tree.Constructor)
	require.NoError(t, err)
	dataSquare := make([][]byte, size*size)
	copy(dataSquare, shares)
	dataSquare[2] = nil
	dataSquare[3] = nil
	dataSquare[8] = nil
	dataSquare[12] = nil
	brokenEDS, err := rsmt2d.RepairExtendedDataSquare(
		attackerEDS.RowRoots(),
		attackerEDS.ColRoots(),
		dataSquare,
		consts.DefaultCodec(),
		tree.Constructor,
	)
	require.Error(t, err)

	// [2] and [3] will be empty
	var errRow *rsmt2d.ErrByzantineRow
	require.True(t, errors.As(err, &errRow))
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*2)
	defer cancel()
	p, err := CreateBadEncodingProof(
		ctx,
		1,
		uint8(errRow.RowNumber),
		true,
		brokenEDS,
		attackerEDS.ColRoots(),
		errRow.Shares,
		dag,
	)

	require.NoError(t, err)
	dah := da.NewDataAvailabilityHeader(attackerEDS)
	h := &header.ExtendedHeader{DAH: &dah}

	err = p.Validate(h)
	require.NoError(t, err)
}

func TestFraudProofValidationForCol(t *testing.T) {
	dag := mdutils.Mock()
	eds := ipld.RandEDS(t, 2)
	size := eds.Width()
	_, err := ipld.PutData(context.Background(), ipld.ExtractODSShares(eds), dag)
	require.NoError(t, err)

	tree := wrapper.NewErasuredNamespacedMerkleTree(uint64(size / 2))
	shares := flatten(eds)
	copy(shares[8][8:], shares[9][8:])

	attackerEDS, err := rsmt2d.ImportExtendedDataSquare(shares, consts.DefaultCodec(), tree.Constructor)
	require.NoError(t, err)

	dataSquare := make([][]byte, len(shares))
	copy(dataSquare, shares)
	dataSquare[1] = nil
	dataSquare[2] = nil
	dataSquare[8] = nil
	dataSquare[12] = nil

	badEds, err := rsmt2d.RepairExtendedDataSquare(
		attackerEDS.RowRoots(),
		attackerEDS.ColRoots(),
		dataSquare,
		consts.DefaultCodec(),
		tree.Constructor,
	)
	require.Error(t, err)

	var errCol *rsmt2d.ErrByzantineCol
	require.True(t, errors.As(err, &errCol))

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*2)
	defer cancel()

	p, err := CreateBadEncodingProof(
		ctx,
		1,
		uint8(errCol.ColNumber),
		false,
		badEds,
		attackerEDS.RowRoots(),
		errCol.Shares,
		dag,
	)
	require.NoError(t, err)

	dah := da.NewDataAvailabilityHeader(attackerEDS)
	h := &header.ExtendedHeader{DAH: &dah}

	err = p.Validate(h)
	require.NoError(t, err)

}

func flatten(eds *rsmt2d.ExtendedDataSquare) [][]byte {
	flattenedEDSSize := eds.Width() * eds.Width()
	out := make([][]byte, flattenedEDSSize)
	count := 0
	for i := uint(0); i < eds.Width(); i++ {
		for _, share := range eds.Row(i) {
			out[count] = share
			count++
		}
	}
	return out
}
