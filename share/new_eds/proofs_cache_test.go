package eds

import (
	"context"
	"testing"
	"time"

	"github.com/celestiaorg/rsmt2d"
)

func TestCache(t *testing.T) {
	size := 8
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	t.Cleanup(cancel)

	withProofsCache := func(tb testing.TB, inner *rsmt2d.ExtendedDataSquare) Accessor {
		accessor := &Rsmt2D{ExtendedDataSquare: inner}
		return WithProofsCache(accessor)
	}

	TestSuiteAccessor(ctx, t, withProofsCache, size)
}
