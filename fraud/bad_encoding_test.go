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

	shares := ipld.ExtractEDS(eds)
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

	dah := &header.ExtendedHeader{DAH: &da}

	p := CreateBadEncodingProof(uint64(dah.Height), errByz)
	err = p.Validate(dah)
	require.NoError(t, err)
}
