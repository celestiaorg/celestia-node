package full

// FIXME: rework testing pkg
// GetterWithRandSquare provides a share.Getter filled with 'n' NMT
// trees of 'n' random shares, essentially storing a whole square.
//func GetterWithRandSquare(t *testing.T, n int) (share.Getter, *share.Root) {
//	bServ := ipld.NewMemBlockservice()
//	getter := getters.NewIPLDGetter(bServ)
//	return getter, availability_test.RandFillBS(t, n, bServ)
//}
//
//// RandNode creates a Full Node filled with a random block of the given size.
//func RandNode(dn *availability_test.TestDagNet, squareSize int) (*availability_test.TestNode, *share.Root) {
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
//func TestAvailability(t *testing.T, getter share.Getter) *ShareAvailability {
//	params := discovery.DefaultParameters()
//	params.AdvertiseInterval = time.Second
//	params.PeersLimit = 10
//	disc, err := discovery.NewDiscovery(
//		params,
//		nil,
//		routing.NewRoutingDiscovery(routinghelpers.Null{}),
//		"full",
//	)
//	require.NoError(t, err)
//	store, err := eds.NewStore(eds.DefaultParameters(), t.TempDir(), datastore.NewMapDatastore())
//	require.NoError(t, err)
//	err = store.Start(context.Background())
//	require.NoError(t, err)
//
//	t.Cleanup(func() {
//		err = store.Stop(context.Background())
//		require.NoError(t, err)
//	})
//	return NewShareAvailability(store, getter, disc)
//}
