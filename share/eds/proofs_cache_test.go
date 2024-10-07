package eds

import (
	"context"
	"testing"
	"time"

	"github.com/celestiaorg/rsmt2d"
)

func TestCache(t *testing.T) {
	ODSSize := 16
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*15)
	t.Cleanup(cancel)

	newAccessor := func(tb testing.TB, inner *rsmt2d.ExtendedDataSquare) Accessor {
		accessor := &Rsmt2D{ExtendedDataSquare: inner}
		return WithProofsCache(accessor)
	}
	TestSuiteAccessor(ctx, t, newAccessor, ODSSize)

	newAccessorStreamer := func(tb testing.TB, inner *rsmt2d.ExtendedDataSquare) AccessorStreamer {
		accessor := &Rsmt2D{ExtendedDataSquare: inner}
		return WithProofsCache(accessor)
	}
	TestStreamer(ctx, t, newAccessorStreamer, ODSSize)
}
