package full

//
//import (
//	"context"
//
//	"testing"
//	"time"
//
//	"github.com/ipfs/go-datastore"
//	"github.com/stretchr/testify/require"
//
//	"github.com/celestiaorg/celestia-node/square"
//	availability_test "github.com/celestiaorg/celestia-node/square/availability/test"
//	"github.com/celestiaorg/celestia-node/square/eds"
//	"github.com/celestiaorg/celestia-node/square/getters"
//	"github.com/celestiaorg/celestia-node/square/ipld"
//	"github.com/celestiaorg/celestia-node/square/p2p/discovery"
//)
//
//// GetterWithRandSquare provides a share.Getter filled with 'n' NMT
//// trees of 'n' random shares, essentially storing a whole square.
//func GetterWithRandSquare(t *testing.T, n int) (square.Getter, *square.AxisRoots) {
//	bServ := ipld.NewMemBlockservice()
//	getter := shwap.NewIPLDGetter(bServ)
//	return getter, availability_test.RandFillBS(t, n, bServ)
//}
//
//// RandNode creates a Full Node filled with a random block of the given size.
//func RandNode(dn *availability_test.TestDagNet, squareSize int) (*availability_test.TestNode, *square.AxisRoots) {
//	nd := Node(dn)
//	return nd, availability_test.RandFillBS(dn.T, squareSize, nd.BlockService)
//}
//
//// Node creates a new empty Full Node.
//func Node(dn *availability_test.TestDagNet) *availability_test.TestNode {
//	nd := dn.NewTestNode()
//	nd.Getter = getters.NewIPLDGetter(nd.BlockService)
//	nd.Availability = TestAvailability(dn.T, nd.Getter)
//	return nd
//}
//
//func TestAvailability(t *testing.T, getter square.Getter) *ShareAvailability {
//	params := discovery.DefaultParameters()
//	params.AdvertiseInterval = time.Second
//	params.PeersLimit = 10
//
//	store, err := eds.NewStore(eds.DefaultParameters(), t.TempDir(), datastore.NewMapDatastore())
//	require.NoError(t, err)
//	err = store.Start(context.Background())
//	require.NoError(t, err)
//
//	t.Cleanup(func() {
//		err = store.Stop(context.Background())
//		require.NoError(t, err)
//	})
//	return NewShareAvailability(store, getter)
//}
