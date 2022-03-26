package fraud

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tendermint/tendermint/pkg/da"
	"github.com/tendermint/tendermint/pkg/wrapper"

	"github.com/celestiaorg/celestia-node/ipld"
	"github.com/celestiaorg/celestia-node/service/header"
	"github.com/celestiaorg/rsmt2d"
)

func TestFraudProof(t *testing.T) {
	eds := ipld.GenerateRandEDS(t, 2)
	shares := ipld.ExtractODSShares(eds)

	tree := wrapper.NewErasuredNamespacedMerkleTree(uint64(2))
	copy(shares[0][8:], shares[1][8:])
	eds1, _ := rsmt2d.ImportExtendedDataSquare(shares, rsmt2d.NewRSGF8Codec(), tree.Constructor)
	dataSquare := make([][]byte, 16)
	dataSquare[0] = shares[0]
	dataSquare[1] = shares[1]
	dataSquare[4] = shares[2]
	dataSquare[5] = shares[3]

	newEds, err := rsmt2d.RepairExtendedDataSquare(
		eds1.RowRoots(),
		eds1.ColRoots(),
		dataSquare,
		rsmt2d.NewRSGF8Codec(),
		tree.Constructor,
	)
	require.Error(t, err)
	require.Nil(t, newEds)
	var errRow *rsmt2d.ErrByzantineRow
	require.True(t, errors.As(err, &errRow))

	errStruct := err.(*rsmt2d.ErrByzantineRow)

	p, err := CreateBadEncodingFraudProof(1, uint8(errStruct.RowNumber), true, newEds, errStruct.Shares)
	require.NoError(t, err)

	dah := da.NewDataAvailabilityHeader(eds1)
	h := &header.ExtendedHeader{DAH: &dah}

	err = p.Validate(h)
	require.NoError(t, err)

}
