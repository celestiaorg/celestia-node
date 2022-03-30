package fraud

import (
	"bytes"
	"errors"
	"fmt"
	"sort"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tendermint/tendermint/pkg/da"

	"github.com/celestiaorg/celestia-node/ipld"
	"github.com/celestiaorg/celestia-node/service/header"
	"github.com/celestiaorg/rsmt2d"
)

func TestFraudProof(t *testing.T) {
	eds := ipld.GenerateRandEDS(t, 2)

	tree := NewErasuredNamespacedMerkleTree(uint64(eds.Width()))
	shares := flatten(eds)

	sortByteArrays(shares)
	copy(shares[0][8:], shares[1][8:])
	eds1, _ := rsmt2d.ImportExtendedDataSquare(shares, rsmt2d.NewRSGF8Codec(), tree.Constructor)

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

	errStruct := err.(*rsmt2d.ErrByzantineRow)
	fmt.Println(errStruct.Shares)
	errStruct.Shares[1] = nil
	errStruct.Shares[3] = nil
	p, err := CreateBadEncodingFraudProof(1, uint8(errStruct.RowNumber), true, newEds, errStruct.Shares)
	require.NoError(t, err)

	dah := da.NewDataAvailabilityHeader(eds1)
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
	fmt.Println(flattenedEDSSize)
	return out
}

func sortByteArrays(src [][]byte) {
	sort.Slice(src, func(i, j int) bool { return bytes.Compare(src[i], src[j]) < 0 })
}
