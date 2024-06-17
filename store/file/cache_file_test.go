package file

import (
	"context"
	"testing"
	"time"

	"github.com/celestiaorg/rsmt2d"

	eds "github.com/celestiaorg/celestia-node/share/new_eds"
)

func TestCacheFile(t *testing.T) {
	size := 8
	newFile := func(inner *rsmt2d.ExtendedDataSquare) eds.Accessor {
		accessor := &eds.Rsmt2D{ExtendedDataSquare: inner}
		return WithProofsCache(accessor)
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	t.Cleanup(cancel)

	eds.TestSuiteAccessor(ctx, t, newFile, size)
}
