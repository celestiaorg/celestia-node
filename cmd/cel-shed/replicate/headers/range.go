package headers

import (
	"context"
	"fmt"

	libhead_store "github.com/celestiaorg/go-header/store"

	"github.com/celestiaorg/celestia-node/header"
)

// resolveRange decides the concrete [start, target] bounds to replicate.
//
// Start defaults to go-header's persisted head + 1; --from-height overrides it.
// Either way the store is contiguous from its base up to its head, so every
// header at or below the head is already on disk: start is clamped up to head+1
// so a repeat or resumed run never re-downloads headers it already holds. Only a
// --from-height strictly above head+1 is honored verbatim — that is a genuine
// forward gap the caller asked for, not a redundant re-fetch.
//
// Target defaults to the source's current head.
func resolveRange(
	ctx context.Context,
	hstore *libhead_store.Store[*header.ExtendedHeader],
	fromFlag, toFlag, srcHeadHeight uint64,
) (uint64, uint64, error) {
	start := fromFlag
	if h, err := hstore.Head(ctx); err == nil && !h.IsZero() {
		if storedNext := h.Height() + 1; start < storedNext {
			start = storedNext
		}
	} else if start == 0 {
		start = 1
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
