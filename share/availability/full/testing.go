package full

import (
	"testing"
	"time"

	routinghelpers "github.com/libp2p/go-libp2p-routing-helpers"
	"github.com/libp2p/go-libp2p/p2p/discovery/routing"

	"github.com/celestiaorg/celestia-node/share"
	availability_test "github.com/celestiaorg/celestia-node/share/availability/test"
	"github.com/celestiaorg/celestia-node/share/getters"
	"github.com/celestiaorg/celestia-node/share/ipld"
	"github.com/celestiaorg/celestia-node/share/p2p/discovery"
)

// GetterWithRandSquare provides a share.Getter filled with 'n' NMT
// trees of 'n' random shares, essentially storing a whole square.
func GetterWithRandSquare(t *testing.T, n int) (share.Getter, *share.Root) {
	bServ := ipld.NewMemBlockservice()
	getter := getters.NewIPLDGetter(bServ)
	return getter, availability_test.RandFillBS(t, n, bServ)
}

type Recoverability int64

const (
	// FullyRecoverable makes all EDS shares available when filling the
	// blockservice.
	FullyRecoverable Recoverability = iota
	// BarelyRecoverable withholds the (k + 1)^2 subsquare of the 2k^2 EDS,
	// minus the node at (k + 1, k + 1) which allows for complete
	// recoverability.
	BarelyRecoverable
	// Unrecoverable withholds the (k + 1)^2 subsquare of the 2k^2 EDS.
	Unrecoverable
)

// RandNode creates a Full Node filled with a random block of the given size.
func RandNode(
	dn *availability_test.TestDagNet,
	squareSize int,
	recoverability Recoverability,
) (*availability_test.TestNode, *share.Root) {
	nd := Node(dn)
	var root *share.Root
	switch recoverability {
	case FullyRecoverable:
		root = availability_test.RandFillBS(dn.T, squareSize, nd.BlockService)
	case BarelyRecoverable:
		root = availability_test.FillWithheldBS(dn.T, squareSize, nd.BlockService, true)
	case Unrecoverable:
		root = availability_test.FillWithheldBS(dn.T, squareSize, nd.BlockService, false)
	default:
		panic("invalid recoverability given")
	}
	return nd, root
}

// Node creates a new empty Full Node.
func Node(dn *availability_test.TestDagNet) *availability_test.TestNode {
	nd := dn.NewTestNode()
	nd.Getter = getters.NewIPLDGetter(nd.BlockService)
	nd.Availability = TestAvailability(nd.Getter)
	return nd
}

func TestAvailability(getter share.Getter) *ShareAvailability {
	disc := discovery.NewDiscovery(
		nil,
		routing.NewRoutingDiscovery(routinghelpers.Null{}),
		discovery.WithAdvertiseInterval(time.Second),
		discovery.WithPeersLimit(10),
	)
	return NewShareAvailability(nil, getter, disc)
}
