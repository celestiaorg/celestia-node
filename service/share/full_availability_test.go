package share

import (
	"context"
	"math/rand"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/sync/errgroup"

	"github.com/celestiaorg/celestia-node/ipld"
)

func init() {
	ipld.RetrieveQuadrantTimeout = time.Millisecond * 100 // to speed up tests
	// randomize quadrant fetching, otherwise quadrant sampling is deterministic
	rand.Seed(time.Now().UnixNano())
}

func TestSharesAvailable_Full(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// RandFullServiceWithSquare creates a NewFullAvailability inside, so we can test it
	service, dah := RandFullServiceWithSquare(t, 16)
	err := service.SharesAvailable(ctx, dah)
	assert.NoError(t, err)
}

func TestShareAvailableOverMocknet_Full(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	net := NewTestDAGNet(ctx, t)
	_, root := net.RandFullNode(32)
	nd := net.FullNode()
	net.ConnectAll()

	err := nd.SharesAvailable(ctx, root)
	assert.NoError(t, err)
}

// TestShareAvailable_OneFullNode asserts that a FullAvailability node can ensure
// data is available(reconstruct data square) while being connected to
// LightAvailability node's only.
func TestShareAvailable_OneFullNode(t *testing.T) {
	// NOTE: Numbers are taken from the original 'Fraud and Data Availability Proofs' paper
	DefaultSampleAmount = 20 // s
	const (
		origSquareSize = 16 // k
		lightNodes     = 69 // c
	)

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Minute)
	defer cancel()

	net := NewTestDAGNet(ctx, t)
	source, root := net.RandFullNode(origSquareSize) // make a source node, a.k.a bridge
	full := net.FullNode()                           // make a full availability service which reconstructs data

	// ensure there is no connection between source and full nodes
	// so that full reconstructs from the light nodes only
	net.Disconnect(source.ID(), full.ID())

	errg, errCtx := errgroup.WithContext(ctx)
	errg.Go(func() error {
		return full.SharesAvailable(errCtx, root)
	})

	lights := make([]*node, lightNodes)
	for i := 0; i < len(lights); i++ {
		lights[i] = net.LightNode()
		go func(i int) {
			err := lights[i].SharesAvailable(ctx, root)
			require.NoError(t, err)
		}(i)
	}

	for i := 0; i < len(lights); i++ {
		net.Connect(lights[i].ID(), source.ID())
	}

	for i := 0; i < len(lights); i++ {
		net.Connect(lights[i].ID(), full.ID())
	}

	err := errg.Wait()
	require.NoError(t, err)
}

// TestShareAvailable_ConnectedFullNodes asserts that two connected FullAvailability nodes
// can ensure data availability via two isolated LightAvailability node subnetworks. Full nodes
// start their availability process first, then Lights start availability process and connect to
// Fulls and only after Lights connect to the source node which has the data. After Lights connect
// to the source, Full must be able to finish the availability process started in the beginning.
func TestShareAvailable_ConnectedFullNodes(t *testing.T) {
	// NOTE: Numbers are taken from the original 'Fraud and Data Availability Proofs' paper
	DefaultSampleAmount = 20 // s
	const (
		origSquareSize = 16 // k
		lightNodes     = 60 // c
	)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*30)
	defer cancel()

	net := NewTestDAGNet(ctx, t)
	source, root := net.RandFullNode(origSquareSize)

	// create two full nodes and ensure they are disconnected
	full1 := net.FullNode()
	full2 := net.FullNode()

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
	lights1, lights2 := make([]*node, lightNodes/2), make([]*node, lightNodes/2)
	for i := 0; i < len(lights1); i++ {
		lights1[i] = net.LightNode()
		go func(i int) {
			err := lights1[i].SharesAvailable(ctx, root)
			require.NoError(t, err)
		}(i)

		lights2[i] = net.LightNode()
		go func(i int) {
			err := lights2[i].SharesAvailable(ctx, root)
			require.NoError(t, err)
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
	require.NoError(t, err)
}

// TestShareAvailable_DisconnectedFullNodes asserts that two disconnected FullAvailability nodes
// cannot ensure data is available (reconstruct data square) while being connected to isolated
// LightAvailability nodes subnetworks, which do not have enough nodes to reconstruct the data,
// but once FullAvailability nodes connect, they can collectively reconstruct it.
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

	// NOTE: Numbers are taken from the original 'Fraud and Data Availability Proofs' paper
	DefaultSampleAmount = 20 // s
	const (
		origSquareSize = 16 // k
		lightNodes     = 60 // c - total number of nodes on two subnetworks
	)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*30)
	defer cancel()

	net := NewTestDAGNet(ctx, t)
	source, root := net.RandFullNode(origSquareSize)

	// create two full nodes and ensure they are disconnected
	full1 := net.FullNode()
	full2 := net.FullNode()
	net.Disconnect(full1.ID(), full2.ID())

	// ensure fulls and source are not connected
	// so that fulls take data from light nodes only
	net.Disconnect(full1.ID(), source.ID())
	net.Disconnect(full2.ID(), source.ID())

	// start reconstruction for fulls that should fail
	ctxErr, cancelErr := context.WithTimeout(ctx, ipld.RetrieveQuadrantTimeout*8)
	errg, errCtx := errgroup.WithContext(ctxErr)
	errg.Go(func() error {
		return full1.SharesAvailable(errCtx, root)
	})
	errg.Go(func() error {
		return full2.SharesAvailable(errCtx, root)
	})

	// create light nodes and start sampling for them immediately
	lights1, lights2 := make([]*node, lightNodes/2), make([]*node, lightNodes/2)
	for i := 0; i < len(lights1); i++ {
		lights1[i] = net.LightNode()
		go func(i int) {
			err := lights1[i].SharesAvailable(ctx, root)
			require.NoError(t, err)
		}(i)

		lights2[i] = net.LightNode()
		go func(i int) {
			err := lights2[i].SharesAvailable(ctx, root)
			require.NoError(t, err)
		}(i)
	}

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

	// check that any of the fulls cannot reconstruct on their own
	err := errg.Wait()
	require.ErrorIs(t, err, ErrNotAvailable)
	cancelErr()

	// but after they connect
	net.Connect(full1.ID(), full2.ID())

	// with clean caches from the previous try
	full1.ClearStorage()
	full2.ClearStorage()

	// they both should be able to reconstruct the block
	err = full1.SharesAvailable(ctx, root)
	require.NoError(t, err, ErrNotAvailable)
	err = full2.SharesAvailable(ctx, root)
	require.NoError(t, err, ErrNotAvailable)
}
