package fraud

import (
	"context"
	"errors"
	"testing"

	format "github.com/ipfs/go-ipld-format"
	mdutils "github.com/ipfs/go-merkledag/test"
	"github.com/stretchr/testify/require"
	"github.com/tendermint/tendermint/pkg/consts"
	"github.com/tendermint/tendermint/pkg/da"
	"github.com/tendermint/tendermint/pkg/wrapper"

	"github.com/celestiaorg/celestia-node/ipld"
	"github.com/celestiaorg/celestia-node/service/header"
	"github.com/celestiaorg/nmt"
	"github.com/celestiaorg/rsmt2d"
)

func TestFraudProofValidation(t *testing.T) {
	dag := mdutils.Mock()
	eds := ipld.RandEDS(t, 2)
	size := eds.Width()

	shares := flatten(eds)
	copy(shares[3][8:], shares[4][8:])
	batchAdder := ipld.NewNmtNodeAdder(
		context.Background(),
		format.NewBatch(context.Background(), dag, format.MaxSizeBatchOption(4)),
	)
	tree := wrapper.NewErasuredNamespacedMerkleTree(uint64(2), nmt.NodeVisitor(batchAdder.Visit))
	attackerEDS, _ := rsmt2d.ImportExtendedDataSquare(shares, consts.DefaultCodec(), tree.Constructor)
	err := batchAdder.Commit()
	require.NoError(t, err)

	dataSquare := make([][]byte, size*size)
	copy(dataSquare, shares)
	dataSquare[2] = nil
	dataSquare[3] = nil
	dataSquare[8] = nil
	dataSquare[12] = nil
	da := da.NewDataAvailabilityHeader(attackerEDS)
	_, err = ipld.RetrieveData(context.Background(), &da, dag, consts.DefaultCodec())
	var errByz *ipld.ErrByzantine
	require.True(t, errors.As(err, &errByz))

	p := CreateBadEncodingProof(1, errByz)

	dah := &header.ExtendedHeader{DAH: &da}

	err = p.Validate(dah)
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
