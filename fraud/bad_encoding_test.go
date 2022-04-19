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
		format.NewBatch(context.Background(), dag, format.MaxSizeBatchOption(int(size)*2)),
	)
	tree := wrapper.NewErasuredNamespacedMerkleTree(uint64(size/2), nmt.NodeVisitor(batchAdder.Visit))
	attackerEDS, _ := rsmt2d.ImportExtendedDataSquare(shares, consts.DefaultCodec(), tree.Constructor)
	err := batchAdder.Commit()
	require.NoError(t, err)

	da := da.NewDataAvailabilityHeader(attackerEDS)
	r := ipld.NewRetriever(dag, consts.DefaultCodec())
	_, err = r.Retrieve(context.Background(), &da)
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

// len: 264, cap: 280, [14,116,82,21,55,141,248,49,123,68,188,5,87,51,193,65,85,139,206,17,235,213,59,65,177,118,57,239,185,38,177,244,193,147,186,31,74,204,47,140,22,223,128,226,131,171,106,22,196,177,240,37,64,88,141,207,80,161,69,251,93,45,144,141,...+200 more]
// []uint8 len: 264, cap: 264, [14,116,82,21,55,141,248,49,150,153,71,135,236,52,254,105,228,222,161,118,70,9,1,12,6,47,215,91,8,155,254,175,63,37,200,82,182,173,36,211,75,29,153,10,174,101,72,19,22,133,15,137,215,163,42,2,146,215,156,255,19,163,125,86,...+200 more]
