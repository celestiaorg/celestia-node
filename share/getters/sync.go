package getters

import (
	"context"
	"fmt"

	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/nmt/namespace"
	"github.com/celestiaorg/rsmt2d"
)

type SyncGetter struct {
	getter share.Getter

	shareSns *sessionBuckets[share.Share]
	edsSns *sessionBuckets[*rsmt2d.ExtendedDataSquare]
	ndSns *sessionBuckets[share.NamespacedShares]
}

func (sg *SyncGetter) GetShare(ctx context.Context, root *share.Root, row, col int) (share.Share, error) {
	key := fmt.Sprintf("%s/%d/%d", root, row, col)

	gtr := func(ctx context.Context) (share.Share, error) {
		return sg.getter.GetShare(ctx, root, row, col)
	}

	return sg.shareSns.getSession(key, gtr).get(ctx)
}

func (sg *SyncGetter) GetEDS(ctx context.Context, root *share.Root) (*rsmt2d.ExtendedDataSquare, error) {
	key := root.String()
	gtr := func(ctx context.Context) (*rsmt2d.ExtendedDataSquare, error) {
		return sg.getter.GetEDS(ctx, root)
	}

	return sg.edsSns.getSession(key, gtr).get(ctx)
}

func (sg *SyncGetter) GetSharesByNamespace(ctx context.Context, root *share.Root, id namespace.ID) (share.NamespacedShares, error) {
	key := root.String() + id.String()
	gtr := func(ctx context.Context) (share.NamespacedShares, error) {
		return sg.getter.GetSharesByNamespace(ctx, root, id)
	}

	return sg.ndSns.getSession(key, gtr).get(ctx)
}
