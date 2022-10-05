package fraud

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/celestiaorg/celestia-node/share"

	mdutils "github.com/ipfs/go-merkledag/test"
	"github.com/stretchr/testify/require"
)

func TestFraudProofValidation(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*15)
	defer t.Cleanup(cancel)
	bServ := mdutils.Bserv()
	_, store := createService(t, false)
	h, err := store.GetByHeight(ctx, 1)
	require.NoError(t, err)

	faultDAH, err := generateByzantineError(ctx, t, h, bServ)
	var errByz *share.ErrByzantine
	require.True(t, errors.As(err, &errByz))
	p := CreateBadEncodingProof([]byte("hash"), uint64(faultDAH.Height), errByz)
	err = p.Validate(faultDAH)
	require.NoError(t, err)
}
