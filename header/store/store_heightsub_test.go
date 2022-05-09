package store

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/celestiaorg/celestia-node/header"
)

func TestHeightSub(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()

	hs := newHeightSub()

	// assert subscription returns nil for past heights
	{
		h := header.RandExtendedHeader(t)
		h.Height = 100
		hs.SetHeight(99)
		hs.Pub(h)

		h, err := hs.Sub(ctx, 10)
		assert.ErrorIs(t, err, errElapsedHeight)
		assert.Nil(t, h)
	}

	// assert actual subscription works
	{
		go func() {
			h1 := header.RandExtendedHeader(t)
			h1.Height = 101
			h2 := header.RandExtendedHeader(t)
			h1.Height = 101
			hs.Pub(h1, h2)
		}()

		h, err := hs.Sub(ctx, 101)
		assert.NoError(t, err)
		assert.NotNil(t, h)
	}
}
