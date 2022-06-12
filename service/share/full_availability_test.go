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
