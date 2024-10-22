// //go:build !race
package full

//
// import (
//	"context"
//	"sync"
//	"testing"
//	"time"
//
//	"github.com/stretchr/testify/require"
//	"golang.org/x/sync/errgroup"
//
//	"github.com/celestiaorg/celestia-node/header/headertest"
//	"github.com/celestiaorg/celestia-node/share"
//	"github.com/celestiaorg/celestia-node/share/availability/light"
//	availability_test "github.com/celestiaorg/celestia-node/share/availability/test"
//	"github.com/celestiaorg/celestia-node/share/eds"
//)
//
// func init() {
//	eds.RetrieveQuadrantTimeout = time.Millisecond * 100 // to speed up tests
//}
//
//// TestShareAvailable_OneFullNode asserts that a full node can ensure
//// data is available (reconstruct data square) while being connected to
//// light nodes only.
// func TestShareAvailable_OneFullNode(t *testing.T) {
//	// NOTE: Numbers are taken from the original 'Fraud and Data Availability Proofs' paper
//	light.DefaultSampleAmount = 20 // s
//	const (
//		origSquareSize = 16 // k
//		lightNodes     = 69 // c
//	)
//
//	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Minute)
//	defer cancel()
//
//	net := availability_test.NewTestDAGNet(ctx, t)
//	source, root := RandNode(net, origSquareSize) // make a source node, a.k.a bridge
//	eh := headertest.RandExtendedHeader(t)
//	eh.DAH = root
//	full := Node(net) // make a full availability service which reconstructs data
//
//	// ensure there is no connection between source and full nodes
//	// so that full reconstructs from the light nodes only
//	net.Disconnect(source.ID(), full.ID())
//
//	errg, errCtx := errgroup.WithContext(ctx)
//	errg.Go(func() error {
//		return full.SharesAvailable(errCtx, eh)
//	})
//
//	lights := make([]*availability_test.TestNode, lightNodes)
//	for i := 0; i < len(lights); i++ {
//		lights[i] = light.Node(net)
//		go func(i int) {
//			err := lights[i].SharesAvailable(ctx, eh)
//			if err != nil {
//				t.Log("light errors:", err)
//			}
//		}(i)
//	}
//
//	for i := 0; i < len(lights); i++ {
//		net.Connect(lights[i].ID(), source.ID())
//	}
//
//	for i := 0; i < len(lights); i++ {
//		net.Connect(lights[i].ID(), full.ID())
//	}
//
//	err := errg.Wait()
//	require.NoError(t, err)
//}
//
//// TestShareAvailable_ConnectedFullNodes asserts that two connected full nodes
//// can ensure data availability via two isolated light node subnetworks. Full
//// nodes start their availability process first, then light node start
//// availability process and connect to full node and only after light node
//// connect to the source node which has the data. After light node connect to the
//// source, full node must be able to finish the availability process started in
//// the beginning.
// func TestShareAvailable_ConnectedFullNodes(t *testing.T) {
//	// NOTE: Numbers are taken from the original 'Fraud and Data Availability Proofs' paper
//	light.DefaultSampleAmount = 20 // s
//	const (
//		origSquareSize = 16 // k
//		lightNodes     = 60 // c
//	)
//
//	ctx, cancel := context.WithTimeout(context.Background(), time.Second*30)
//	defer cancel()
//
//	net := availability_test.NewTestDAGNet(ctx, t)
//	source, root := RandNode(net, origSquareSize)
//	eh := headertest.RandExtendedHeader(t)
//	eh.DAH = root
//
//	// create two full nodes and ensure they are disconnected
//	full1 := Node(net)
//	full2 := Node(net)
//
//	// pre-connect fulls
//	net.Connect(full1.ID(), full2.ID())
//	// ensure fulls and source are not connected
//	// so that fulls take data from light nodes only
//	net.Disconnect(full1.ID(), source.ID())
//	net.Disconnect(full2.ID(), source.ID())
//
//	// start reconstruction for fulls
//	errg, errCtx := errgroup.WithContext(ctx)
//	errg.Go(func() error {
//		return full1.SharesAvailable(errCtx, eh)
//	})
//	errg.Go(func() error {
//		return full2.SharesAvailable(errCtx, eh)
//	})
//
//	// create light nodes and start sampling for them immediately
//	lights1, lights2 := make(
//		[]*availability_test.TestNode, lightNodes/2),
//		make([]*availability_test.TestNode, lightNodes/2)
//	for i := 0; i < len(lights1); i++ {
//		lights1[i] = light.Node(net)
//		go func(i int) {
//			err := lights1[i].SharesAvailable(ctx, eh)
//			if err != nil {
//				t.Log("light1 errors:", err)
//			}
//		}(i)
//
//		lights2[i] = light.Node(net)
//		go func(i int) {
//			err := lights2[i].SharesAvailable(ctx, eh)
//			if err != nil {
//				t.Log("light2 errors:", err)
//			}
//		}(i)
//	}
//
//	// shape topology
//	for i := 0; i < len(lights1); i++ {
//		// ensure lights1 are only connected to full1
//		net.Connect(lights1[i].ID(), full1.ID())
//		net.Disconnect(lights1[i].ID(), full2.ID())
//		// ensure lights2 are only connected to full2
//		net.Connect(lights2[i].ID(), full2.ID())
//		net.Disconnect(lights2[i].ID(), full1.ID())
//	}
//
//	// start connection lights with sources
//	for i := 0; i < len(lights1); i++ {
//		net.Connect(lights1[i].ID(), source.ID())
//		net.Connect(lights2[i].ID(), source.ID())
//	}
//
//	err := errg.Wait()
//	require.NoError(t, err)
//}
//
//// TestShareAvailable_DisconnectedFullNodes asserts that two disconnected full
//// nodes cannot ensure data is available (reconstruct data square) while being
//// connected to isolated light nodes subnetworks, which do not have enough nodes
//// to reconstruct the data, but once ShareAvailability nodes connect, they can
//// collectively reconstruct it.
////
////nolint:dupword
// func TestShareAvailable_DisconnectedFullNodes(t *testing.T) {
//	// S - Source
//	// L - Light Node
//	// F - Full Node
//	// ── - connection
//	//
//	// Topology:
//	// NOTE: There are more Light Nodes in practice
//	// ┌─┬─┬─S─┬─┬─┐
//	// │ │ │   │ │ │
//	// │ │ │   │ │ │
//	// │ │ │   │ │ │
//	// L L L   L L L
//	// │ │ │   │ │ │
//	// └─┴─┤   ├─┴─┘
//	//    F└───┘F
//	//
//
//	// NOTE: Numbers are taken from the original 'Fraud and Data Availability Proofs' paper
//	light.DefaultSampleAmount = 20 // s
//	const (
//		origSquareSize = 16 // k
//		lightNodes     = 32 // c - total number of nodes on two subnetworks
//	)
//
//	ctx, cancel := context.WithTimeout(context.Background(), time.Second*60)
//	defer cancel()
//
//	net := availability_test.NewTestDAGNet(ctx, t)
//	source, root := RandNode(net, origSquareSize)
//	eh := headertest.RandExtendedHeader(t)
//	eh.DAH = root
//
//	// create light nodes and start sampling for them immediately
//	lights1, lights2 := make(
//		[]*availability_test.TestNode, lightNodes/2),
//		make([]*availability_test.TestNode, lightNodes/2)
//
//	var wg sync.WaitGroup
//	wg.Add(lightNodes)
//	for i := 0; i < len(lights1); i++ {
//		lights1[i] = light.Node(net)
//		go func(i int) {
//			defer wg.Done()
//			err := lights1[i].SharesAvailable(ctx, eh)
//			if err != nil {
//				t.Log("light1 errors:", err)
//			}
//		}(i)
//
//		lights2[i] = light.Node(net)
//		go func(i int) {
//			defer wg.Done()
//			err := lights2[i].SharesAvailable(ctx, eh)
//			if err != nil {
//				t.Log("light2 errors:", err)
//			}
//		}(i)
//	}
//
//	// create two full nodes and ensure they are disconnected
//	full1 := Node(net)
//	full2 := Node(net)
//	net.Disconnect(full1.ID(), full2.ID())
//
//	// ensure fulls and source are not connected
//	// so that fulls take data from light nodes only
//	net.Disconnect(full1.ID(), source.ID())
//	net.Disconnect(full2.ID(), source.ID())
//
//	// shape topology
//	for i := 0; i < len(lights1); i++ {
//		// ensure lights1 are only connected to source and full1
//		net.Connect(lights1[i].ID(), source.ID())
//		net.Connect(lights1[i].ID(), full1.ID())
//		net.Disconnect(lights1[i].ID(), full2.ID())
//		// ensure lights2 are only connected to source and full2
//		net.Connect(lights2[i].ID(), source.ID())
//		net.Connect(lights2[i].ID(), full2.ID())
//		net.Disconnect(lights2[i].ID(), full1.ID())
//	}
//
//	// start reconstruction for fulls that should fail
//	ctxErr, cancelErr := context.WithTimeout(ctx, time.Second*5)
//	errg, errCtx := errgroup.WithContext(ctxErr)
//	errg.Go(func() error {
//		return full1.SharesAvailable(errCtx, eh)
//	})
//	errg.Go(func() error {
//		return full2.SharesAvailable(errCtx, eh)
//	})
//
//	// check that any of the fulls cannot reconstruct on their own
//	err := errg.Wait()
//	require.ErrorIs(t, err, share.ErrNotAvailable)
//	cancelErr()
//
//	// but after they connect
//	net.Connect(full1.ID(), full2.ID())
//
//	// with clean caches from the previous try
//	full1.ClearStorage()
//	full2.ClearStorage()
//
//	// they both should be able to reconstruct the block
//	errg, bctx := errgroup.WithContext(ctx)
//	errg.Go(func() error {
//		return full1.SharesAvailable(bctx, eh)
//	})
//	errg.Go(func() error {
//		return full2.SharesAvailable(bctx, eh)
//	})
//	require.NoError(t, errg.Wait())
//	// wait for all routines to finish before exit, in case there are any errors to log
//	wg.Wait()
//}
