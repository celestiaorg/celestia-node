package headers

import (
	"context"
	"fmt"

	libhead_store "github.com/celestiaorg/go-header/store"

	"github.com/celestiaorg/celestia-node/header"
)

// resolveRange decides the concrete [start, target] bounds to replicate.
// Start defaults to go-header's persisted head + 1; --from-height overrides.
// Target defaults to the source's current head.
func resolveRange(
	ctx context.Context,
	hstore *libhead_store.Store[*header.ExtendedHeader],
	fromFlag, toFlag, srcHeadHeight uint64,
) (uint64, uint64, error) {
	start := fromFlag
	if start == 0 {
		if h, err := hstore.Head(ctx); err == nil && !h.IsZero() {
			start = h.Height() + 1
		} else {
			start = 1
		}
	}

	target := toFlag
	if target == 0 {
		target = srcHeadHeight
	}
	if target > srcHeadHeight {
		return 0, 0, fmt.Errorf("to-height %d > source head %d", target, srcHeadHeight)
	}
	return start, target, nil
}
