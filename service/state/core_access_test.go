package state

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/cosmos/cosmos-sdk/crypto/hd"
	"github.com/cosmos/cosmos-sdk/crypto/keyring"
	"github.com/cosmos/cosmos-sdk/testutil"
	"github.com/stretchr/testify/require"

	apputil "github.com/celestiaorg/celestia-app/testutil"
	apptypes "github.com/celestiaorg/celestia-app/x/payment/types"
	"github.com/celestiaorg/celestia-node/core"
)

func TestCoreAccess(t *testing.T) {
	// create signer + acct
	dir := t.TempDir()
	ring, err := keyring.New("celestia", "test", dir, os.Stdin)
	require.NoError(t, err)
	acc, err := ring.NewAccount("something", testutil.TestMnemonic, "", "", hd.Secp256k1)
	require.NoError(t, err)
	signer := apptypes.NewKeyringSigner(ring, acc.GetName(), "test")

	// create app and start core node
	celapp := apputil.SetupTestApp(t, acc.GetAddress())
	nd := core.StartMockNode(celapp)
	defer nd.Stop() //nolint:errcheck
	_, ip := core.GetRemoteEndpoint(nd)
	grpcEndpoint := fmt.Sprintf("%s:9090", ip)

	// create CoreAccess with the grpc endpoint to mock core node
	ca := NewCoreAccessor(signer, grpcEndpoint)
	bal, err := ca.Balance(context.Background())
	require.NoError(t, err)
	t.Log("BAL: ", bal)
}
