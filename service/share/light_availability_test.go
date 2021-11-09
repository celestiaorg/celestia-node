package share

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/celestiaorg/celestia-node/service/header"
)

func TestSharesAvailability(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// RandServiceWithSquare creates a NewLightAvailability inside, so we can test it
	service, dah := RandServiceWithSquare(t, 16)
	err := service.SharesAvailable(ctx, dah)
	assert.NoError(t, err)
}

func TestSharesAvailabilityFailed(t *testing.T) {
	DefaultSamples = 2

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// RandServiceWithSquare creates a NewLightAvailability inside, so we can test it
	s, _ := RandServiceWithSquare(t, 16)
	empty := header.EmptyDAH()
	err := s.SharesAvailable(ctx, &empty)
	assert.Error(t, err)
}
