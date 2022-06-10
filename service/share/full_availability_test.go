package share

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
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

	net := NewDAGNet(ctx, t)
	_, root := net.RandFullService(16)
	serv := net.CleanService()
	net.ConnectAll()

	err := serv.SharesAvailable(ctx, root)
	assert.NoError(t, err)
}
