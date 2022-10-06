package full

import (
	"context"
	"math/rand"
	"testing"
	"time"

	availability_test "github.com/celestiaorg/celestia-node/share/availability/test"

	"github.com/stretchr/testify/assert"
)

func init() {
	// randomize quadrant fetching, otherwise quadrant sampling is deterministic
	rand.Seed(time.Now().UnixNano())
}

func TestShareAvailableOverMocknet_Full(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	net := availability_test.NewTestDAGNet(ctx, t)
	_, root := RandNode(net, 32)
	nd := Node(net)
	net.ConnectAll()

	err := nd.SharesAvailable(ctx, root)
	assert.NoError(t, err)
}

func TestSharesAvailable_Full(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// RandServiceWithSquare creates a NewShareAvailability inside, so we can test it
	service, dah := RandServiceWithSquare(t, 16)
	err := service.SharesAvailable(ctx, dah)
	assert.NoError(t, err)
}
