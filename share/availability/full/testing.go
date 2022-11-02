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
	"github.com/celestiaorg/celestia-node/share/service"
)

// RandServiceWithSquare provides a service.ShareService filled with 'n' NMT
// trees of 'n' random shares, essentially storing a whole square.
func RandServiceWithSquare(t *testing.T, n int) (*service.ShareService, *share.Root, error) {
	bServ := mdutils.Bserv()
	fa, err := TestAvailability(bServ)
	if err != nil {
		return nil, nil, err
	}
	return service.NewShareService(bServ, fa), availability_test.RandFillBS(t, n, bServ), nil
}

// RandNode creates a Full Node filled with a random block of the given size.
func RandNode(dn *availability_test.TestDagNet, squareSize int) (*availability_test.TestNode, *share.Root, error) {
	nd, err := Node(dn)
	if err != nil {
		return nil, nil, err
	}
	return nd, availability_test.RandFillBS(dn.T, squareSize, nd.BlockService), nil
}

// Node creates a new empty Full Node.
func Node(dn *availability_test.TestDagNet) (*availability_test.TestNode, error) {
	nd := dn.NewTestNode()
	fa, err := TestAvailability(nd.BlockService)
	if err != nil {
		return nil, err
	}
	nd.ShareService = service.NewShareService(nd.BlockService, fa)
	return nd, nil
}

func TestAvailability(bServ blockservice.BlockService) (*ShareAvailability, error) {
	disc, err := discovery.NewDiscovery(
		nil,
		routing.NewRoutingDiscovery(routinghelpers.Null{}),
		availability.WithPeersLimit(0),
		availability.WithDiscoveryInterval(time.Second),
		availability.WithAdvertiseInterval(time.Second),
	)
	if err != nil {
		return nil, err
	}
	return NewShareAvailability(bServ, disc)
}
