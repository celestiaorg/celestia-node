package full

import (
	"context"
	"testing"
	"time"

	"github.com/ipfs/go-blockservice"
	"github.com/ipfs/go-datastore"
	ds_sync "github.com/ipfs/go-datastore/sync"
	mdutils "github.com/ipfs/go-merkledag/test"
	"github.com/libp2p/go-libp2p-core/host"
	routinghelpers "github.com/libp2p/go-libp2p-routing-helpers"
	"github.com/libp2p/go-libp2p/p2p/discovery/routing"
	"github.com/stretchr/testify/require"

	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/celestia-node/share/availability/discovery"
	availability_test "github.com/celestiaorg/celestia-node/share/availability/test"
	"github.com/celestiaorg/celestia-node/share/eds"
	"github.com/celestiaorg/celestia-node/share/eds/p2p"
	"github.com/celestiaorg/celestia-node/share/service"
)

// RandServiceWithSquare provides a service.ShareService filled with 'n' NMT
// trees of 'n' random shares, essentially storing a whole square.
func RandServiceWithSquare(t *testing.T, n int) (*service.ShareService, *share.Root) {
	bServ := mdutils.Bserv()
	return service.NewShareService(bServ, TestAvailability(t, bServ, nil)), availability_test.RandFillBS(t, n, bServ)
}

// RandNode creates a Full Node filled with a random block of the given size.
func RandNode(dn *availability_test.TestDagNet, squareSize int) (*availability_test.TestNode, *share.Root) {
	nd := Node(dn)
	return nd, availability_test.RandFillBS(dn.T, squareSize, nd.BlockService)
}

// Node creates a new empty Full Node.
func Node(dn *availability_test.TestDagNet) *availability_test.TestNode {
	nd := dn.NewTestNode()
	nd.ShareService = service.NewShareService(nd.BlockService, TestAvailability(dn.T, nd.BlockService, nd.Host))
	return nd
}

func TestAvailability(t *testing.T, bServ blockservice.BlockService, host host.Host) *ShareAvailability {
	disc := discovery.NewDiscovery(host, routing.NewRoutingDiscovery(routinghelpers.Null{}), 0, time.Second, time.Second)

	ds := ds_sync.MutexWrap(datastore.NewMapDatastore())
	store, err := eds.NewStore(t.TempDir(), ds)
	require.NoError(t, err)
	err = store.Start(context.Background())
	require.NoError(t, err)

	client := p2p.NewClient(host, "")
	return NewShareAvailability(bServ, disc, client, store)
}
