package state

import (
	"context"
	"testing"

	sdkmath "cosmossdk.io/math"
	sdktypes "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/celestiaorg/celestia-app/v10/pkg/user"
	libshare "github.com/celestiaorg/go-square/v4/share"

	"github.com/celestiaorg/celestia-node/state/txclient"
)

// grantFeeStubTxClient is a minimal TxClient that never gets called for the
// invalid-amount cases below, since GrantFee is expected to reject the
// amount before submitting anything.
type grantFeeStubTxClient struct{}

func (grantFeeStubTxClient) SubmitMessage(
	context.Context, sdktypes.Msg, *txclient.TxConfig,
) (*user.TxResponse, error) {
	return &user.TxResponse{Height: 1}, nil
}

func (grantFeeStubTxClient) SubmitPayForBlob(
	context.Context, []*libshare.Blob, sdktypes.AccAddress, *txclient.TxConfig,
) (*user.TxResponse, error) {
	return &user.TxResponse{Height: 1}, nil
}

// TestGrantFeeInvalidAmount verifies that GrantFee rejects a nil or negative
// amount instead of panicking in sdk.NewCoin, matching the validation already
// done by Transfer, Delegate, Undelegate, and the other write methods. Zero
// must still be accepted: it means an unlimited spend limit.
func TestGrantFeeInvalidAmount(t *testing.T) {
	ca := &CoreAccessor{
		txClient:             grantFeeStubTxClient{},
		defaultSignerAddress: sdktypes.AccAddress(make([]byte, 20)),
	}
	ctx := context.Background()
	cfg := txclient.NewTxConfig()
	grantee := sdktypes.AccAddress(make([]byte, 20))

	testCases := []struct {
		name    string
		amount  sdkmath.Int
		wantErr bool
	}{
		{name: "nil amount", amount: sdkmath.Int{}, wantErr: true},
		{name: "negative amount", amount: sdkmath.NewInt(-1), wantErr: true},
		{name: "zero amount means unlimited", amount: sdkmath.NewInt(0), wantErr: false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var (
				resp *TxResponse
				err  error
			)
			require.NotPanics(t, func() {
				resp, err = ca.GrantFee(ctx, grantee, tc.amount, cfg)
			})
			if tc.wantErr {
				assert.ErrorIs(t, err, ErrInvalidAmount)
				return
			}
			require.NoError(t, err)
			require.EqualValues(t, 1, resp.Height)
		})
	}
}
