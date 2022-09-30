package full

import (
	"testing"
	"time"

	"github.com/ipfs/go-blockservice"
	mdutils "github.com/ipfs/go-merkledag/test"
	routinghelpers "github.com/libp2p/go-libp2p-routing-helpers"
	"github.com/libp2p/go-libp2p/p2p/discovery/routing"

	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/celestia-node/share/availability"
	"github.com/celestiaorg/celestia-node/share/availability/discovery"
	availability_test "github.com/celestiaorg/celestia-node/share/availability/test"
)

// randFullServiceWithSquare provides an availability.Service filled with 'n' NMT
// trees of 'n' random shares, essentially storing a whole square.
func randFullServiceWithSquare(t *testing.T, n int) (*availability.Service, *share.Root) {
	bServ := mdutils.Bserv()
	return availability.NewService(bServ, TestFullAvailability(bServ)), availability_test.RandFillBS(t, n, bServ)
}

// RandFullNode creates a Full Node filled with a random block of the given size.
func RandFullNode(dn *availability_test.DagNet, squareSize int) (*availability_test.Node, *share.Root) {
	nd := Node(dn)
	return nd, availability_test.RandFillBS(dn.T, squareSize, nd.BlockService)
}

// Node creates a new empty Full Node.
func Node(dn *availability_test.DagNet) *availability_test.Node {
	nd := dn.Node()
	nd.Service = availability.NewService(nd.BlockService, TestFullAvailability(nd.BlockService))
	return nd
}

func TestFullAvailability(bServ blockservice.BlockService) *ShareAvailability {
	disc := discovery.NewDiscovery(nil, routing.NewRoutingDiscovery(routinghelpers.Null{}), 0, time.Second, time.Second)
	return NewShareAvailability(bServ, disc)
}

func SubNetNode(sn *availability_test.SubNet) *availability_test.Node {
	nd := Node(sn.DagNet)
	sn.AddNode(nd)
	return nd
}
