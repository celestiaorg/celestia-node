package light

import (
	"testing"
	"time"

	"github.com/celestiaorg/celestia-node/share/service"

	"github.com/ipfs/go-blockservice"
	mdutils "github.com/ipfs/go-merkledag/test"
	routinghelpers "github.com/libp2p/go-libp2p-routing-helpers"
	"github.com/libp2p/go-libp2p/p2p/discovery/routing"

	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/celestia-node/share/availability/discovery"
	availability_test "github.com/celestiaorg/celestia-node/share/availability/test"
)

// randLightServiceWithSquare provides a share.Service filled with 'n' NMT
// trees of 'n' random shares, essentially storing a whole square.
func randLightServiceWithSquare(t *testing.T, n int) (*service.ShareService, *share.Root) {
	bServ := mdutils.Bserv()

	return service.NewShareService(bServ, TestLightAvailability(bServ)), availability_test.RandFillBS(t, n, bServ)
}

// randLightService provides an unfilled share.Service with corresponding
// blockservice.BlockService than can be filled by the test.
func randLightService() (*service.ShareService, blockservice.BlockService) {
	bServ := mdutils.Bserv()
	return service.NewShareService(bServ, TestLightAvailability(bServ)), bServ
}

// randLightNode creates a Light Node filled with a random block of the given size.
func randLightNode(dn *availability_test.DagNet, squareSize int) (*availability_test.Node, *share.Root) {
	nd := Node(dn)
	return nd, availability_test.RandFillBS(dn.T, squareSize, nd.BlockService)
}

// Node creates a new empty Light Node.
func Node(dn *availability_test.DagNet) *availability_test.Node {
	nd := dn.Node()
	nd.ShareService = service.NewShareService(nd.BlockService, TestLightAvailability(nd.BlockService))
	return nd
}

func TestLightAvailability(bServ blockservice.BlockService) *ShareAvailability {
	disc := discovery.NewDiscovery(nil, routing.NewRoutingDiscovery(routinghelpers.Null{}), 0, time.Second, time.Second)
	return NewLightAvailability(bServ, disc)
}

func SubNetNode(sn *availability_test.SubNet) *availability_test.Node {
	nd := Node(sn.DagNet)
	sn.AddNode(nd)
	return nd
}
