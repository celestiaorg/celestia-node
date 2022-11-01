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
func RandServiceWithSquare(t *testing.T, n int) (*service.ShareService, *share.Root, error) {
	bServ := mdutils.Bserv()
	la, err := TestAvailability(bServ)
	if err != nil {
		return nil, nil, err
	}
	return service.NewShareService(bServ, la), availability_test.RandFillBS(t, n, bServ), nil
}

// RandService provides an unfilled share.Service with corresponding
// blockservice.BlockService than can be filled by the test.
func RandService() (*service.ShareService, blockservice.BlockService, error) {
	bServ := mdutils.Bserv()
	la, err := TestAvailability(bServ)
	if err != nil {
		return nil, nil, err
	}
	return service.NewShareService(bServ, la), bServ, nil
}

// RandNode creates a Light Node filled with a random block of the given size.
func RandNode(dn *availability_test.TestDagNet, squareSize int) (*availability_test.TestNode, *share.Root, error) {
	nd, err := Node(dn)
	if err != nil {
		return nil, nil, err
	}
	return nd, availability_test.RandFillBS(dn.T, squareSize, nd.BlockService), nil
}

// Node creates a new empty Light Node.
func Node(dn *availability_test.TestDagNet) (*availability_test.TestNode, error) {
	nd := dn.NewTestNode()
	la, err := TestAvailability(nd.BlockService)
	if err != nil {
		return nil, err
	}
	nd.ShareService = service.NewShareService(nd.BlockService, la)
	return nd, nil
}

func TestAvailability(bServ blockservice.BlockService) (*ShareAvailability, error) {
	disc := discovery.NewDiscovery(nil, routing.NewRoutingDiscovery(routinghelpers.Null{}), 0, time.Second, time.Second)
	return NewShareAvailability(bServ, disc)
}

func SubNetNode(sn *availability_test.SubNet) (*availability_test.TestNode, error) {
	nd, err := Node(sn.TestDagNet)
	if err != nil {
		return nil, err
	}
	sn.AddNode(nd)
	return nd, nil
}
