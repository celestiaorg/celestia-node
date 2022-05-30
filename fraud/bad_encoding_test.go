package fraud

import (
	"context"
	"errors"
	"testing"

	mdutils "github.com/ipfs/go-merkledag/test"
	"github.com/stretchr/testify/require"
	"github.com/tendermint/tendermint/pkg/da"

	"github.com/celestiaorg/celestia-node/header"
	"github.com/celestiaorg/celestia-node/ipld"
)

func TestFraudProofValidation(t *testing.T) {
	bServ := mdutils.Bserv()
	eds := ipld.RandEDS(t, 2)

	shares := ipld.ExtractEDS(eds)
	copy(shares[3][8:], shares[4][8:])
	eds, err := ipld.ImportShares(context.Background(), shares, bServ)
	require.NoError(t, err)
	da := da.NewDataAvailabilityHeader(eds)
	r := ipld.NewRetriever(bServ)
	_, err = r.Retrieve(context.Background(), &da)
	var errByz *ipld.ErrByzantine
	require.True(t, errors.As(err, &errByz))

	dah := &header.ExtendedHeader{DAH: &da}

	p := CreateBadEncodingProof(uint64(dah.Height), errByz)
	err = p.Validate(dah)
	require.NoError(t, err)
}
