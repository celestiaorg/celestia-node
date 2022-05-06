// TODO(@Wondertan): Ideally, we should move that into subpackage, so this does not get included into binary of
//  production code, but that does not matter at the moment.
package header

import (
	"context"
	"testing"

	extheader "github.com/celestiaorg/celestia-node/service/header/extHeader"

	"github.com/ipfs/go-datastore"
	"github.com/ipfs/go-datastore/sync"
	"github.com/stretchr/testify/require"
)

// NewTestStore creates initialized and started in memory header Store which is useful for testing.
func NewTestStore(ctx context.Context, t *testing.T, head *extheader.ExtendedHeader) Store {
	store, err := NewStoreWithHead(ctx, sync.MutexWrap(datastore.NewMapDatastore()), head)
	require.NoError(t, err)

	err = store.Start(ctx)
	require.NoError(t, err)

	t.Cleanup(func() {
		err := store.Stop(ctx)
		require.NoError(t, err)
	})
	return store
}

type DummySubscriber struct {
	Headers []*extheader.ExtendedHeader
}

func (mhs *DummySubscriber) AddValidator(Validator) error {
	return nil
}

func (mhs *DummySubscriber) Subscribe() (Subscription, error) {
	return mhs, nil
}

func (mhs *DummySubscriber) NextHeader(ctx context.Context) (*extheader.ExtendedHeader, error) {
	defer func() {
		if len(mhs.Headers) > 1 {
			// pop the already-returned header
			cp := mhs.Headers
			mhs.Headers = cp[1:]
		} else {
			mhs.Headers = make([]*extheader.ExtendedHeader, 0)
		}
	}()
	if len(mhs.Headers) == 0 {
		return nil, context.Canceled
	}
	return mhs.Headers[0], nil
}

func (mhs *DummySubscriber) Cancel() {}
