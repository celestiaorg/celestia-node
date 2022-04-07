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
	net.Disconnect(0, 1)

	err := full.SharesAvailable(ctx, root)
	assert.NoError(t, err)
}
