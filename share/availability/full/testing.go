package full

import (
	"testing"
	"time"

	mdutils "github.com/ipfs/go-merkledag/test"
	routinghelpers "github.com/libp2p/go-libp2p-routing-helpers"
	"github.com/libp2p/go-libp2p/p2p/discovery/routing"

	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/celestia-node/share/availability/discovery"
	availability_test "github.com/celestiaorg/celestia-node/share/availability/test"
	"github.com/celestiaorg/celestia-node/share/getters"
)

// GetterWithRandSquare provides a share.Getter filled with 'n' NMT
// trees of 'n' random shares, essentially storing a whole square.
func GetterWithRandSquare(t *testing.T, n int) (share.Getter, *share.Root) {
	bServ := mdutils.Bserv()
	getter := getters.NewIPLDGetter(bServ)
	return getter, availability_test.RandFillBS(t, n, bServ)
}

// RandNode creates a Full Node filled with a random block of the given size.
func RandNode(dn *availability_test.TestDagNet, squareSize int) (*availability_test.TestNode, *share.Root) {
	nd := Node(dn)
	return nd, availability_test.RandFillBS(dn.T, squareSize, nd.BlockService)
}

// Node creates a new empty Full Node.
func Node(dn *availability_test.TestDagNet) *availability_test.TestNode {
	nd := dn.NewTestNode()
	nd.Getter = getters.NewIPLDGetter(nd.BlockService)
	nd.Availability = TestAvailability(nd.Getter)
	return nd
}

func TestAvailability(getter share.Getter) *ShareAvailability {
	disc := discovery.NewDiscovery(nil, routing.NewRoutingDiscovery(
		routinghelpers.Null{}), 0, time.Second, time.Second,
	)
	return NewShareAvailability(nil, getter, disc)
}
