package light

import (
	"testing"
	"time"

	"github.com/ipfs/go-blockservice"
	mdutils "github.com/ipfs/go-merkledag/test"
	routinghelpers "github.com/libp2p/go-libp2p-routing-helpers"
	"github.com/libp2p/go-libp2p/p2p/discovery/routing"

	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/celestia-node/share/availability/discovery"
	availability_test "github.com/celestiaorg/celestia-node/share/availability/test"
	"github.com/celestiaorg/celestia-node/share/service"
)

// RandServiceWithSquare provides a share.Service filled with 'n' NMT
// trees of 'n' random shares, essentially storing a whole square.
func RandServiceWithSquare(t *testing.T, n int) (*ShareAvailability, *share.Root) {
	bServ := mdutils.Bserv()

	shareServ, dah := service.NewShareService(bServ), availability_test.RandFillBS(t, n, bServ)
	return NewShareAvailability(shareServ, bServ, RandDisc()), dah
}

func RandDisc() *discovery.Discovery {
	return &discovery.Discovery{}
}

// RandService provides an unfilled share.Service with corresponding
// blockservice.BlockService than can be filled by the test.
func RandService() (service.ShareService, blockservice.BlockService) {
	bServ := mdutils.Bserv()
	return service.NewShareService(bServ), bServ
}

// RandNode creates a Light Node filled with a random block of the given size.
func RandNode(dn *availability_test.TestDagNet, squareSize int) (*availability_test.TestNode, *share.Root) {
	nd := Node(dn)
	return nd, availability_test.RandFillBS(dn.T, squareSize, nd.BlockService)
}

// Node creates a new empty Light Node.
func Node(dn *availability_test.TestDagNet) *availability_test.TestNode {
	nd := dn.NewTestNode()
	nd.ShareService = service.NewShareService(nd.BlockService)
	nd.Availability = NewShareAvailability(nd.ShareService, nd.BlockService, RandDisc())
	return nd
}

func TestAvailability(bServ blockservice.BlockService) *ShareAvailability {
	disc := discovery.NewDiscovery(nil, routing.NewRoutingDiscovery(routinghelpers.Null{}), 0, time.Second, time.Second)
	return NewShareAvailability(service.NewShareService(bServ), bServ, disc)
}

func SubNetNode(sn *availability_test.SubNet) *availability_test.TestNode {
	nd := Node(sn.TestDagNet)
	sn.AddNode(nd)
	return nd
}
