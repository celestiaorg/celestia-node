package share

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/celestiaorg/celestia-node/header"
)

func TestSharesAvailable(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// randLightServiceWithSquare creates a NewLightAvailability inside, so we can test it
	service, dah := randLightServiceWithSquare(t, 16)
	err := service.SharesAvailable(ctx, dah)
	assert.NoError(t, err)
}

func TestSharesAvailableFailed(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// randLightServiceWithSquare creates a NewLightAvailability inside, so we can test it
	s, _ := randLightServiceWithSquare(t, 16)
	empty := header.EmptyDAH()
	err := s.SharesAvailable(ctx, &empty)
	assert.Error(t, err)
}

func TestShareAvailableOverMocknet(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	net := NewTestDAGNet(ctx, t)
	_, root := net.RandLightNode(16)
	nd := net.LightNode()
	net.ConnectAll()

	err := nd.SharesAvailable(ctx, root)
	assert.NoError(t, err)
}
