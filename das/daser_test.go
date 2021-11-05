package das

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/celestiaorg/celestia-node/service/header"
	"github.com/celestiaorg/celestia-node/service/share"
)

func TestDASer(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	s, dah := share.RandServiceWithSquare(t, 16)
	das := NewDASer(s)

	err := das.DAS(ctx, dah)
	assert.NoError(t, err)
}

func TestDASFailed(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	s, _ := share.RandServiceWithSquare(t, 16)
	das := NewDASer(s)

	DefaultSamples = 2
	empty := header.EmptyDAH()
	err := das.DAS(ctx, &empty)
	assert.Error(t, err)
}
