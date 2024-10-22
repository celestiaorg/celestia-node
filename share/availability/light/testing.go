package light

//
// import (
//	"context"
//	"sync"
//	"testing"
//
//	"github.com/ipfs/boxo/blockservice"
//	"github.com/ipfs/go-datastore"
//
//	"github.com/celestiaorg/rsmt2d"
//
//	"github.com/celestiaorg/celestia-node/header"
//	"github.com/celestiaorg/celestia-node/header/headertest"
//	"github.com/celestiaorg/celestia-node/share"
//	availability_test "github.com/celestiaorg/celestia-node/share/availability/test"
//	"github.com/celestiaorg/celestia-node/share/getters"
//	"github.com/celestiaorg/celestia-node/share/ipld"
//)
//
//// GetterWithRandSquare provides a share.Getter filled with 'n' NMT trees of 'n' random shares,
//// essentially storing a whole square.
// func GetterWithRandSquare(t *testing.T, n int) (share.Getter, *header.ExtendedHeader) {
//	bServ := ipld.NewMemBlockservice()
//	getter := getters.NewIPLDGetter(bServ)
//	root := availability_test.RandFillBS(t, n, bServ)
//	eh := headertest.RandExtendedHeader(t)
//	eh.DAH = root
//
//	return getter, eh
//}
//
//// EmptyGetter provides an unfilled share.Getter with corresponding blockservice.BlockService than
//// can be filled by the test.
// func EmptyGetter() (share.Getter, blockservice.BlockService) {
//	bServ := ipld.NewMemBlockservice()
//	getter := getters.NewIPLDGetter(bServ)
//	return getter, bServ
//}
//
//// RandNode creates a Light Node filled with a random block of the given size.
// func RandNode(dn *availability_test.TestDagNet, squareSize int) (*availability_test.TestNode, *share.AxisRoots) {
//	nd := Node(dn)
//	return nd, availability_test.RandFillBS(dn.T, squareSize, nd.BlockService)
//}
//
//// Node creates a new empty Light Node.
// func Node(dn *availability_test.TestDagNet) *availability_test.TestNode {
//	nd := dn.NewTestNode()
//	nd.Getter = getters.NewIPLDGetter(nd.BlockService)
//	nd.Availability = TestAvailability(nd.Getter)
//	return nd
//}
//
// func TestAvailability(getter share.Getter) *ShareAvailability {
//	ds := datastore.NewMapDatastore()
//	return NewShareAvailability(getter, ds)
//}
//
// func SubNetNode(sn *availability_test.SubNet) *availability_test.TestNode {
//	nd := Node(sn.TestDagNet)
//	sn.AddNode(nd)
//	return nd
//}
//
// type onceGetter struct {
//	*sync.Mutex
//	available map[Sample]struct{}
//}
//
// func newOnceGetter() onceGetter {
//	return onceGetter{
//		Mutex:     &sync.Mutex{},
//		available: make(map[Sample]struct{}),
//	}
//}
//
// func (m onceGetter) AddSamples(samples []Sample) {
//	m.Lock()
//	defer m.Unlock()
//	for _, s := range samples {
//		m.available[s] = struct{}{}
//	}
//}
//
// func (m onceGetter) GetShare(_ context.Context, _ *header.ExtendedHeader, row, col int) (libshare.Share, error) {
//	m.Lock()
//	defer m.Unlock()
//	s := Sample{Row: uint16(row), Col: uint16(col)}
//	if _, ok := m.available[s]; ok {
//		delete(m.available, s)
//		return libshare.Share{}, nil
//	}
//	return libshare.Share{}, share.ErrNotAvailable
//}
//
// func (m onceGetter) GetEDS(_ context.Context, _ *header.ExtendedHeader) (*rsmt2d.ExtendedDataSquare, error) {
//	panic("not implemented")
//}
//
// func (m onceGetter) GetSharesByNamespace(
//	_ context.Context,
//	_ *header.ExtendedHeader,
//	_ libshare.Namespace,
// ) (libshare.NamespacedShares, error) {
//	panic("not implemented")
// }
