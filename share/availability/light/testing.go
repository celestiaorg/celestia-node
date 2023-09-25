package light

import (
	"testing"

	"github.com/ipfs/boxo/blockservice"
	"github.com/ipfs/go-datastore"

	"github.com/celestiaorg/celestia-node/share"
	availability_test "github.com/celestiaorg/celestia-node/share/availability/test"
	"github.com/celestiaorg/celestia-node/share/getters"
	"github.com/celestiaorg/celestia-node/share/ipld"
)

// GetterWithRandSquare provides a share.Getter filled with 'n' NMT trees of 'n' random shares,
// essentially storing a whole square.
func GetterWithRandSquare(t *testing.T, n int) (share.Getter, *share.Root) {
	bServ := ipld.NewMemBlockservice()
	getter := getters.NewIPLDGetter(bServ)
	return getter, availability_test.RandFillBS(t, n, bServ)
}

// EmptyGetter provides an unfilled share.Getter with corresponding blockservice.BlockService than
// can be filled by the test.
func EmptyGetter() (share.Getter, blockservice.BlockService) {
	bServ := ipld.NewMemBlockservice()
	getter := getters.NewIPLDGetter(bServ)
	return getter, bServ
}

// RandNode creates a Light Node filled with a random block of the given size.
func RandNode(dn *availability_test.TestDagNet, squareSize int) (*availability_test.TestNode, *share.Root) {
	nd := Node(dn)
	return nd, availability_test.RandFillBS(dn.T, squareSize, nd.BlockService)
}

// Node creates a new empty Light Node.
func Node(dn *availability_test.TestDagNet) *availability_test.TestNode {
	nd := dn.NewTestNode()
	nd.Getter = getters.NewIPLDGetter(nd.BlockService)
	nd.Availability = TestAvailability(nd.Getter)
	return nd
}

func TestAvailability(getter share.Getter) *ShareAvailability {
	ds := datastore.NewMapDatastore()
	return NewShareAvailability(getter, ds)
}

func SubNetNode(sn *availability_test.SubNet) *availability_test.TestNode {
	nd := Node(sn.TestDagNet)
	sn.AddNode(nd)
	return nd
}
