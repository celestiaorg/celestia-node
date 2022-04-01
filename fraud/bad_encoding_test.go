package fraud

import (
	"bytes"
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tendermint/tendermint/pkg/da"

	"github.com/celestiaorg/celestia-node/ipld"
	"github.com/celestiaorg/celestia-node/service/header"
	"github.com/celestiaorg/rsmt2d"
)

func TestCreateBadEncodingFraudProofFromRows(t *testing.T) {
	eds := ipld.GenerateRandEDS(t, 2)
	type test struct {
		name   string
		isRow  bool
		roots  func(uint) [][]byte
		length int
	}
	tests := []test{
		{"rowRoots", true, eds.Row, len(eds.RowRoots())},
		{"colRoots", false, eds.Col, len(eds.ColRoots())},
	}
	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			for index := 0; index < tc.length; index++ {
				p, err := CreateBadEncodingFraudProof(1, uint8(index), tc.isRow, eds, tc.roots(uint(index)))
				require.NoError(t, err)
				dah := da.NewDataAvailabilityHeader(eds)
				h := &header.ExtendedHeader{DAH: &dah}
				require.NoError(t, p.Validate(h))
			}
		})
	}
}

func TestFraudProof(t *testing.T) {
	eds := ipld.GenerateRandEDS(t, 2)
	size := eds.Width()

	tree := NewErasuredNamespacedMerkleTree(uint64(size / 2))
	shares := flatten(eds)

	copy(shares[0][8:], shares[1][8:])
	eds1, err := rsmt2d.ImportExtendedDataSquare(shares, rsmt2d.NewRSGF8Codec(), tree.Constructor)
	require.NoError(t, err)
	newEds, err := rsmt2d.RepairExtendedDataSquare(
		eds1.RowRoots(),
		eds1.ColRoots(),
		shares,
		rsmt2d.NewRSGF8Codec(),
		tree.Constructor,
	)
	require.Error(t, err)
	var errRow *rsmt2d.ErrByzantineRow
	require.True(t, errors.As(err, &errRow))

	errRow = err.(*rsmt2d.ErrByzantineRow)
	fmt.Println(errRow.Shares)

	p, err := CreateBadEncodingFraudProof(1, uint8(errRow.RowNumber), true, newEds, errRow.Shares)
	require.NoError(t, err)

	dah := da.NewDataAvailabilityHeader(eds1)
	h := &header.ExtendedHeader{DAH: &dah}

	err = p.Validate(h)
	require.NoError(t, err)

}

func TestBuildTreeFromDataRoot(t *testing.T) {
	eds := ipld.GenerateRandEDS(t, 8)
	size := eds.Width()
	type test struct {
		name      string
		length    int
		errShares func(uint) [][]byte
		data      func(uint) [][]byte
		roots     [][]byte
	}
	var tests = []test{
		{
			"build tree from row roots",
			len(eds.RowRoots()),
			eds.Row,
			eds.Col,
			eds.ColRoots(),
		},
		{
			"build tree from col roots",
			len(eds.ColRoots()),
			eds.Col,
			eds.Row,
			eds.RowRoots(),
		},
	}
	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			for i := 0; i < tc.length; i++ {
				errShares := tc.errShares(uint(i))
				for index := range errShares {
					tree := NewErasuredNamespacedMerkleTree(uint64(size / 2))
					require.NoError(t, buildTreeFromDataRoot(&tree, tc.data(uint(index)), index))
					require.True(t, bytes.Equal(tc.roots[index], tree.Root()))
				}
			}
		})
	}
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
