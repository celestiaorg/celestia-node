package share

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/sync/errgroup"

	"github.com/celestiaorg/celestia-node/ipld"
)

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

func TestShareAvailable_FullOverLights(t *testing.T) {
	ipld.RetrieveQuadrantTimeout = time.Millisecond * 200
	DefaultSampleAmount = 20 // s

	const (
		origSquareSize = 16 // k
		lightNodes     = 69 // c
	)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*60)
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

	errg, errCtx := errgroup.WithContext(ctx)
	for i := 0; i < len(lights); i++ {
		i := i
		errg.Go(func() error {
			return lights[i].SharesAvailable(errCtx, root)
		})
	}

	err := errg.Wait()
	require.NoError(t, err)

	// ensure there is no connection between source and full nodes
	// so that full reconstructs from the light nodes only
	net.Disconnect(source.ID(), full.ID())

	err = full.SharesAvailable(ctx, root)
	assert.NoError(t, err)
}
