//go:build !race

package full

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"golang.org/x/sync/errgroup"

	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/celestia-node/share/availability/light"
	availability_test "github.com/celestiaorg/celestia-node/share/availability/test"
	"github.com/celestiaorg/celestia-node/share/eds"
)

func init() {
	eds.RetrieveQuadrantTimeout = time.Millisecond * 100 // to speed up tests
}

// TestShareAvailable_OneFullNode asserts that a full node can ensure
// data is available (reconstruct data square) while being connected to
// light nodes only.
func TestShareAvailable_OneFullNode(t *testing.T) {
	// NOTE: Numbers are taken from the original 'Fraud and Data Availability Proofs' paper
	tc := []struct {
		name            string
		origSquareSize  int  // k
		lightNodes      int  // c
		sampleAmount    uint // s
		recoverability  Recoverability
		expectedFailure bool
	}{
		{
			name:            "fully recoverable",
			origSquareSize:  16,
			lightNodes:      24, // ~99% chance of recoverability
			sampleAmount:    20,
			recoverability:  FullyRecoverable,
			expectedFailure: false,
		},
		{
			name:            "fully recoverable, not enough LNs",
			origSquareSize:  16,
			lightNodes:      19, // ~0.7% chance of recoverability
			sampleAmount:    20,
			recoverability:  FullyRecoverable,
			expectedFailure: true,
		},
		{
			name:            "barely recoverable",
			origSquareSize:  16,
			lightNodes:      230, // 99% chance of recoverability
			sampleAmount:    20,
			recoverability:  BarelyRecoverable,
			expectedFailure: false,
		},
		{
			name:            "barely recoverable, not enough LNs",
			origSquareSize:  16,
			lightNodes:      22, // ~0.3% chance of recoverability
			sampleAmount:    20,
			recoverability:  BarelyRecoverable,
			expectedFailure: true,
		},
		{
			name:            "unrecoverable",
			origSquareSize:  16,
			lightNodes:      230,
			sampleAmount:    20,
			recoverability:  Unrecoverable,
			expectedFailure: true,
		},
	}

	for _, tt := range tc {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			light.DefaultSampleAmount = tt.sampleAmount // s
			origSquareSize := tt.origSquareSize         // k
			lightNodes := tt.lightNodes                 // c

			ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
			defer cancel()

			net := availability_test.NewTestDAGNet(ctx, t)
			// make a source node, a.k.a bridge
			source, root := RandNode(net, origSquareSize, tt.recoverability)
			// make a full availability service which reconstructs data
			full := Node(net)

			// ensure there is no connection between source and full nodes
			// so that full reconstructs from the light nodes only
			net.Disconnect(source.ID(), full.ID())

			lights := make([]*availability_test.TestNode, lightNodes)
			for i := 0; i < len(lights); i++ {
				lights[i] = light.Node(net)
				// Form topology
				net.Connect(lights[i].ID(), source.ID())
				net.Connect(lights[i].ID(), full.ID())
				// Start sampling
				go func(i int) {
					err := lights[i].SharesAvailable(ctx, root)
					if err != nil {
						t.Log("light errors:", err)
					}
				}(i)
			}

			err := full.SharesAvailable(ctx, root)
			if tt.expectedFailure {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

// TestShareAvailable_ConnectedFullNodes asserts that two connected full nodes
// can ensure data availability via two isolated light node subnetworks. Full
// nodes start their availability process first, then light node start
// availability process and connect to full node and only after light node
// connect to the source node which has the data. After light node connect to the
// source, full node must be able to finish the availability process started in
// the beginning.
func TestShareAvailable_ConnectedFullNodes(t *testing.T) {
	tc := []struct {
		name            string
		origSquareSize  int  // k
		lightNodes      int  // c
		sampleAmount    uint // s
		recoverability  Recoverability
		expectedFailure bool
	}{
		{
			name:            "fully recoverable",
			origSquareSize:  16,
			lightNodes:      24,
			sampleAmount:    20,
			recoverability:  FullyRecoverable,
			expectedFailure: false,
		},
		{
			name:            "fully recoverable, not enough LNs",
			origSquareSize:  16,
			lightNodes:      19, // ~0.7% chance of recoverability
			sampleAmount:    20,
			recoverability:  FullyRecoverable,
			expectedFailure: true,
		},
		// NOTE: This test contains cases for barely recoverable but
		// DisconnectedFullNodes does not.  The reasoning for this is that
		// DisconnectedFullNodes has the additional contstraint that the data
		// should not be reconstructable from a single subnetwork, while this
		// test only tests that the data is reconstructable once the subnetworks
		// are connected.
		{
			name:            "barely recoverable",
			origSquareSize:  16,
			lightNodes:      230, // 99% chance of recoverability
			sampleAmount:    20,
			recoverability:  BarelyRecoverable,
			expectedFailure: false,
		},
		{
			name:            "barely recoverable, not enough LNs",
			origSquareSize:  16,
			lightNodes:      22, // ~0.3% chance of recoverability
			sampleAmount:    20,
			recoverability:  BarelyRecoverable,
			expectedFailure: true,
		},
	}

	for _, tt := range tc {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			light.DefaultSampleAmount = tt.sampleAmount // s
			origSquareSize := tt.origSquareSize         // k
			lightNodes := tt.lightNodes                 // c

			ctx, cancel := context.WithTimeout(context.Background(), time.Second*60)
			defer cancel()

			net := availability_test.NewTestDAGNet(ctx, t)
			source, root := RandNode(net, origSquareSize, tt.recoverability)

			// create two full nodes and ensure they are disconnected
			full1 := Node(net)
			full2 := Node(net)

			// pre-connect fulls
			net.Connect(full1.ID(), full2.ID())
			// ensure fulls and source are not connected
			// so that fulls take data from light nodes only
			net.Disconnect(full1.ID(), source.ID())
			net.Disconnect(full2.ID(), source.ID())

			// start reconstruction for fulls
			errg, errCtx := errgroup.WithContext(ctx)
			errg.Go(func() error {
				return full1.SharesAvailable(errCtx, root)
			})
			errg.Go(func() error {
				return full2.SharesAvailable(errCtx, root)
			})

			// create light nodes and start sampling for them immediately
			lights1, lights2 := make(
				[]*availability_test.TestNode, lightNodes/2),
				make([]*availability_test.TestNode, lightNodes/2)
			for i := 0; i < len(lights1); i++ {
				lights1[i] = light.Node(net)
				go func(i int) {
					err := lights1[i].SharesAvailable(ctx, root)
					if err != nil {
						t.Log("light1 errors:", err)
					}
				}(i)

				lights2[i] = light.Node(net)
				go func(i int) {
					err := lights2[i].SharesAvailable(ctx, root)
					if err != nil {
						t.Log("light2 errors:", err)
					}
				}(i)
			}

			// shape topology
			for i := 0; i < len(lights1); i++ {
				// ensure lights1 are only connected to full1
				net.Connect(lights1[i].ID(), full1.ID())
				net.Disconnect(lights1[i].ID(), full2.ID())
				// ensure lights2 are only connected to full2
				net.Connect(lights2[i].ID(), full2.ID())
				net.Disconnect(lights2[i].ID(), full1.ID())
			}

			// start connection lights with sources
			for i := 0; i < len(lights1); i++ {
				net.Connect(lights1[i].ID(), source.ID())
				net.Connect(lights2[i].ID(), source.ID())
			}

			err := errg.Wait()
			if tt.expectedFailure {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

// TestShareAvailable_DisconnectedFullNodes asserts that two disconnected full
// nodes cannot ensure data is available (reconstruct data square) while being
// connected to isolated light nodes subnetworks, which do not have enough nodes
// to reconstruct the data, but once ShareAvailability nodes connect, they can
// collectively reconstruct it.
func TestShareAvailable_DisconnectedFullNodes(t *testing.T) {
	// S - Source
	// L - Light Node
	// F - Full Node
	// ── - connection
	//
	// Topology:
	// NOTE: There are more Light Nodes in practice
	// ┌─┬─┬─S─┬─┬─┐
	// │ │ │   │ │ │
	// │ │ │   │ │ │
	// │ │ │   │ │ │
	// L L L   L L L
	// │ │ │   │ │ │
	// └─┴─┤   ├─┴─┘
	//    F└───┘F
	//
	tc := []struct {
		name            string
		origSquareSize  int  // k
		lightNodes      int  // c
		sampleAmount    uint // s
		recoverability  Recoverability
		expectedFailure bool
	}{
		// NOTE: The number of LNs must be even, otherwise the WaitGroup will hang.
		{
			name:            "fully recoverable",
			origSquareSize:  16,
			lightNodes:      24, // ~99% chance of recoverability
			sampleAmount:    20,
			recoverability:  FullyRecoverable,
			expectedFailure: false,
		},
		{
			name:            "fully recoverable, not enough LNs",
			origSquareSize:  16,
			lightNodes:      18, // ~0.03% chance of recoverability
			sampleAmount:    20,
			recoverability:  FullyRecoverable,
			expectedFailure: true,
		},
		{
			name:            "unrecoverable",
			origSquareSize:  16,
			lightNodes:      230,
			sampleAmount:    20,
			recoverability:  Unrecoverable,
			expectedFailure: true,
		},
	}
	for _, tt := range tc {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			light.DefaultSampleAmount = tt.sampleAmount // s
			origSquareSize := tt.origSquareSize         // k
			lightNodes := tt.lightNodes                 // c

			ctx, cancel := context.WithTimeout(context.Background(), time.Second*60)
			defer cancel()

			net := availability_test.NewTestDAGNet(ctx, t)
			source, root := RandNode(net, origSquareSize, tt.recoverability)

			// create light nodes and start sampling for them immediately
			lights1, lights2 := make(
				[]*availability_test.TestNode, lightNodes/2),
				make([]*availability_test.TestNode, lightNodes/2)

			var wg sync.WaitGroup
			wg.Add(lightNodes)
			for i := 0; i < len(lights1); i++ {
				lights1[i] = light.Node(net)
				go func(i int) {
					defer wg.Done()
					err := lights1[i].SharesAvailable(ctx, root)
					if err != nil {
						t.Log("light1 errors:", err)
					}
				}(i)

				lights2[i] = light.Node(net)
				go func(i int) {
					defer wg.Done()
					err := lights2[i].SharesAvailable(ctx, root)
					if err != nil {
						t.Log("light2 errors:", err)
					}
				}(i)
			}

			// create two full nodes and ensure they are disconnected
			full1 := Node(net)
			full2 := Node(net)
			net.Disconnect(full1.ID(), full2.ID())

			// ensure fulls and source are not connected
			// so that fulls take data from light nodes only
			net.Disconnect(full1.ID(), source.ID())
			net.Disconnect(full2.ID(), source.ID())

			// shape topology
			for i := 0; i < len(lights1); i++ {
				// ensure lights1 are only connected to source and full1
				net.Connect(lights1[i].ID(), source.ID())
				net.Connect(lights1[i].ID(), full1.ID())
				net.Disconnect(lights1[i].ID(), full2.ID())
				// ensure lights2 are only connected to source and full2
				net.Connect(lights2[i].ID(), source.ID())
				net.Connect(lights2[i].ID(), full2.ID())
				net.Disconnect(lights2[i].ID(), full1.ID())
			}

			// start reconstruction for fulls that should fail
			ctxErr, cancelErr := context.WithTimeout(ctx, time.Second*5)
			errg, errCtx := errgroup.WithContext(ctxErr)
			errg.Go(func() error {
				err := full1.SharesAvailable(errCtx, root)
				if err == nil {
					return errors.New("full1 should not be able to reconstruct")
				}
				// this is a trick to ensure that BOTH fulls fail with this error using a single errgroup.
				if err != share.ErrNotAvailable {
					return err
				}
				return nil
			})
			errg.Go(func() error {
				err := full2.SharesAvailable(errCtx, root)
				if err == nil {
					return errors.New("full2 should not be able to reconstruct")
				}
				// this is a trick to ensure that BOTH fulls fail with this error using a single errgroup.
				if err != share.ErrNotAvailable {
					return err
				}
				return nil
			})

			// check that any of the fulls cannot reconstruct on their own
			err := errg.Wait()
			require.NoError(t, err)
			cancelErr()

			// but after they connect
			net.Connect(full1.ID(), full2.ID())

			// we clear the blockservices not because we need to, but just to
			// show its possible to reconstruct without any previously saved
			// data from previous attempts.
			full1.ClearStorage()
			full2.ClearStorage()

			// they both should be able to reconstruct the block
			errg, bctx := errgroup.WithContext(ctx)
			errg.Go(func() error {
				return full1.SharesAvailable(bctx, root)
			})
			errg.Go(func() error {
				return full2.SharesAvailable(bctx, root)
			})

			err = errg.Wait()
			if tt.expectedFailure {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
			// wait for all routines to finish before exit, in case there are any errors to log
			wg.Wait()
		})
	}

}
