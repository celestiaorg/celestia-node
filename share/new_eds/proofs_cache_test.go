package eds

import (
	"context"
	"testing"
	"time"

	"github.com/celestiaorg/rsmt2d"
)

func TestCache(t *testing.T) {
	size := 8
	withProofsCache := func(inner *rsmt2d.ExtendedDataSquare) Accessor {
		accessor := &Rsmt2D{ExtendedDataSquare: inner}
		return WithProofsCache(accessor)
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	t.Cleanup(cancel)

	TestSuiteAccessor(ctx, t, withProofsCache, size)
}
