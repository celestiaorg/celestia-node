package utils

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCtxWithStartupBuffer(t *testing.T) {
	t.Run("no parent deadline returns parent unchanged", func(t *testing.T) {
		ctx, cancel := CtxWithStartupBuffer(context.Background())
		defer cancel()
		_, ok := ctx.Deadline()
		assert.False(t, ok)
	})

	t.Run("far-future deadline is buffered earlier", func(t *testing.T) {
		parentDL := time.Now().Add(time.Hour)
		pctx, pcancel := context.WithDeadline(context.Background(), parentDL)
		defer pcancel()

		bctx, bcancel := CtxWithStartupBuffer(pctx)
		defer bcancel()

		innerDL, ok := bctx.Deadline()
		require.True(t, ok)
		assert.WithinDuration(t, parentDL.Add(-StartupErrorBuffer), innerDL, 10*time.Millisecond)
	})

	t.Run("deadline within buffer returns parent unchanged", func(t *testing.T) {
		sctx, scancel := context.WithTimeout(context.Background(), StartupErrorBuffer/2)
		defer scancel()

		rctx, rcancel := CtxWithStartupBuffer(sctx)
		defer rcancel()

		sDL, _ := sctx.Deadline()
		rDL, ok := rctx.Deadline()
		require.True(t, ok)
		assert.Equal(t, sDL, rDL)
	})
}
