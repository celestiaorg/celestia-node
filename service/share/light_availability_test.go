package share

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/celestiaorg/celestia-node/service/header"
)

func TestDASer(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	s, dah := RandServiceWithSquare(t, 16)
	das := NewLightAvailability(s)

	err := das.SharesAvailable(ctx, dah)
	assert.NoError(t, err)
}

func TestDASFailed(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	s, _ := RandServiceWithSquare(t, 16)
	das := NewLightAvailability(s)

	DefaultSamples = 2
	empty := header.EmptyDAH()
	err := das.SharesAvailable(ctx, &empty)
	assert.Error(t, err)
}
