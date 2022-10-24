package ipld_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/celestiaorg/celestia-node/share/availability/full"
	availability_test "github.com/celestiaorg/celestia-node/share/availability/test"
	"github.com/celestiaorg/celestia-node/share/service"
)

// TestNamespaceHasher_CorruptedData is an integration test that verifies that the NamespaceHasher of a recipient of
// corrupted data will not panic, and will throw away the corrupted data.
func TestNamespaceHasher_CorruptedData(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	net := availability_test.NewTestDAGNet(ctx, t)

	requestor := full.Node(net)
	provider, mockBS := availability_test.MockNode(t, net)
	provider.ShareService = service.NewShareService(provider.BlockService, full.TestAvailability(provider.BlockService))
	net.ConnectAll()

	// before the provider starts attacking, we should be able to retrieve successfully
	root := availability_test.RandFillBS(t, 16, provider.BlockService)
	getCtx, cancelGet := context.WithTimeout(ctx, 2*time.Second)
	t.Cleanup(cancelGet)
	err := requestor.SharesAvailable(getCtx, root)
	require.NoError(t, err)

	// clear the storage of the requestor so that it must retrieve again, then start attacking
	requestor.ClearStorage()
	mockBS.Attacking = true
	getCtx, cancelGet = context.WithTimeout(ctx, 2*time.Second)
	t.Cleanup(cancelGet)
	err = requestor.SharesAvailable(getCtx, root)
	require.Error(t, err, "availability validation failed")
}
