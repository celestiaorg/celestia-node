package state

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	netutil "github.com/celestiaorg/celestia-app/testutil/network"
	"github.com/celestiaorg/celestia-app/x/payment/types"
)

func TestCoreAccess(t *testing.T) {
	conf := netutil.DefaultConfig()
	net := netutil.New(t, conf, "test1", "test2")
	t.Cleanup(net.Cleanup)

	port := net.Validators[0].AppConfig.GRPC.Address[7:]
	endpoint := fmt.Sprintf("127.0.0.1%s", port)

	keys, err := net.Validators[0].ClientCtx.Keyring.List()
	require.NoError(t, err)

	signer := types.NewKeyringSigner(net.Validators[0].ClientCtx.Keyring, keys[1].GetName(), "test")
	ca := NewCoreAccessor(signer, endpoint)

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	err = ca.Start(ctx)
	require.NoError(t, err)
	t.Cleanup(func() {
		ca.Stop(ctx) //nolint:errcheck
	})

	bal, err := ca.Balance(ctx)
	require.NoError(t, err)
	assert.Equal(t, "0CELES", bal.String())

	// build tx
	builder := signer.NewTxBuilder()
	namespace := []byte{1, 1, 1, 1, 1, 1, 1, 1}
	message := []byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 0}
	msg, err := types.NewWirePayForMessage(namespace, message, 4, 16, 32)
	require.NoError(t, err)

	tx, err := signer.BuildSignedTx(builder, msg)
	require.NoError(t, err)

	raw, err := signer.EncodeTx(tx)
	require.NoError(t, err)

	resp, err := ca.SubmitTx(ctx, raw)
	require.NoError(t, err)
	t.Log(resp)

}
