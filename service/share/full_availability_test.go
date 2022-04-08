package share

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/celestiaorg/celestia-node/service/header"
)

func TestSharesAvailable_Full(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// RandFullServiceWithSquare creates a NewFullAvailability inside, so we can test it
	service, dah := RandFullServiceWithSquare(t, 16)
	err := service.SharesAvailable(ctx, dah)
	assert.NoError(t, err)
}

func TestSharesAvailableFailed_Full(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// RandFullServiceWithSquare creates a NewFullAvailability inside, so we can test it
	s, _ := RandFullServiceWithSquare(t, 16)
	empty := header.EmptyDAH()
	err := s.SharesAvailable(ctx, &empty)
	assert.Error(t, err)
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

func TestShareAvailable_FullOverLights(t *testing.T) {
	const (
		origSquareSize = 8
		lightNodes     = 192
	)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*30)
	defer cancel()

	net := NewTestDAGNet(ctx, t)
	source, root := net.RandFullNode(origSquareSize) // make a source node, a.k.a bridge
	full := net.FullNode()                           // make a full availability service which reconstructs data

	lights := make([]*node, lightNodes)
	for i := 0; i < len(lights); i++ {
		light := net.LightNode()
		net.Connect(light.ID(), full.ID())
		net.Connect(light.ID(), source.ID())
		lights[i] = light
	}

	for i := 0; i < len(lights); i++ {
		err := lights[i].SharesAvailable(ctx, root)
		require.NoError(t, err)
	}

	// ensure there is no connection between source and full nodes
	// so that full reconstructs from the light nodes only
	net.Disconnect(source.ID(), full.ID())

	err := full.SharesAvailable(ctx, root)
	assert.NoError(t, err)
}

func TestShareAvailable_MultipleFullOverLights(t *testing.T) {
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
	const (
		origSquareSize = 8
		lightNodes     = 128 // total number of nodes on two subnetworks
	)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*30)
	defer cancel()

	net := NewTestDAGNet(ctx, t)
	source, root := net.RandFullNode(origSquareSize)

	full1 := net.FullNode()
	lights1 := make([]*node, lightNodes/2)
	for i := 0; i < len(lights1); i++ {
		light1 := net.LightNode()
		net.Connect(light1.ID(), source.ID())
		net.Connect(light1.ID(), full1.ID())
		lights1[i] = light1
	}

	full2 := net.FullNode()
	lights2 := make([]Service, lightNodes/2)
	for i := 0; i < len(lights2); i++ {
		light2 := net.LightNode()
		net.Connect(light2.ID(), source.ID())
		net.Connect(light2.ID(), full2.ID())
		lights2[i] = light2
	}

	for i := 0; i < len(lights1); i++ {
		err := lights1[i].SharesAvailable(ctx, root)
		require.NoError(t, err)
	}

	for i := 0; i < len(lights2); i++ {
		err := lights2[i].SharesAvailable(ctx, root)
		require.NoError(t, err)
	}

	// ensure full and source are not connected
	net.Disconnect(full1.ID(), source.ID())
	net.Disconnect(full2.ID(), source.ID())

	ctx1, cancel1 := context.WithTimeout(ctx, time.Second*3)
	defer cancel1()

	// check that full1 cannot reconstruct on its own
	err := full1.SharesAvailable(ctx1, root)
	require.ErrorIs(t, err, ErrNotAvailable)

	ctx2, cancel2 := context.WithTimeout(ctx, time.Second*3)
	defer cancel2()

	// check that full2 cannot reconstruct on its own
	err = full2.SharesAvailable(ctx2, root)
	require.ErrorIs(t, err, ErrNotAvailable)

	// but after they connect
	net.Connect(full1.ID(), full2.ID())

	// they both should be able to reconstruct the block
	err = full1.SharesAvailable(ctx, root)
	require.NoError(t, err, ErrNotAvailable)
	err = full2.SharesAvailable(ctx, root)
	require.NoError(t, err, ErrNotAvailable)
}
