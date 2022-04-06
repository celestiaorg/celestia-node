package share

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/celestiaorg/celestia-node/service/header"
)

func TestSharesAvailable(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// RandLightServiceWithSquare creates a NewLightAvailability inside, so we can test it
	service, dah := RandLightServiceWithSquare(t, 16)
	err := service.SharesAvailable(ctx, dah)
	assert.NoError(t, err)
}

func TestSharesAvailableFailed(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// RandLightServiceWithSquare creates a NewLightAvailability inside, so we can test it
	s, _ := RandLightServiceWithSquare(t, 16)
	empty := header.EmptyDAH()
	err := s.SharesAvailable(ctx, &empty)
	assert.Error(t, err)
}

func TestShareAvailableOverMocknet(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	net := NewDAGNet(ctx, t)
	_, root := net.RandLightService(16)
	serv := net.CleanLightService()
	net.ConnectAll()

	err := serv.SharesAvailable(ctx, root)
	assert.NoError(t, err)
}
