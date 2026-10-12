package header

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	libhead "github.com/celestiaorg/go-header"
	"github.com/celestiaorg/go-header/sync"

	"github.com/celestiaorg/celestia-node/header"
	"github.com/celestiaorg/celestia-node/header/headertest"
)

func TestGetByHeightHandlesError(t *testing.T) {
	serv := Service{
		syncer: &errorSyncer[*header.ExtendedHeader]{},
	}

	assert.NotPanics(t, func() {
		h, err := serv.GetByHeight(context.Background(), 100)
		assert.Error(t, err)
		assert.Nil(t, h)
	})
}

type errorSyncer[H libhead.Header[H]] struct{}

func (d *errorSyncer[H]) Head(context.Context, ...libhead.HeadOption[H]) (H, error) {
	var zero H
	return zero, fmt.Errorf("dummy error")
}

func (d *errorSyncer[H]) State() sync.State {
	return sync.State{}
}

func (d *errorSyncer[H]) SyncWait(context.Context) error {
	return fmt.Errorf("dummy error")
}

func TestGetByHeightSyncerHeadBehindStore(t *testing.T) {
	ctx := context.Background()
	store := headertest.NewStore(t)
	storeHead, err := store.Head(ctx)
	require.NoError(t, err)
	staleHead, err := store.GetByHeight(ctx, storeHead.Height()-5)
	require.NoError(t, err)

	serv := Service{
		syncer: &staleSyncer{head: staleHead},
		store:  store,
	}

	for _, height := range []uint64{storeHead.Height(), storeHead.Height() - 1} {
		expected, err := store.GetByHeight(ctx, height)
		require.NoError(t, err)
		h, err := serv.GetByHeight(ctx, height)
		require.NoError(t, err)
		assert.Equal(t, expected.Hash(), h.Hash())
	}

	// The test store returns ErrNotFound instead of waiting for the next header.
	_, err = serv.GetByHeight(ctx, storeHead.Height()+1)
	assert.ErrorIs(t, err, libhead.ErrNotFound)

	_, err = serv.GetByHeight(ctx, storeHead.Height()+2)
	assert.ErrorContains(t, err, "given height is from the future")

	storeErr := errors.New("store unavailable")
	serv.store = &headErrorStore{Store: store, err: storeErr}
	_, err = serv.GetByHeight(ctx, storeHead.Height())
	assert.ErrorIs(t, err, storeErr)
}

type headErrorStore struct {
	libhead.Store[*header.ExtendedHeader]
	err error
}

func (s *headErrorStore) Head(context.Context, ...libhead.HeadOption[*header.ExtendedHeader]) (*header.ExtendedHeader, error) {
	return nil, s.err
}

type staleSyncer struct {
	errorSyncer[*header.ExtendedHeader]
	head *header.ExtendedHeader
}

func (s *staleSyncer) Head(
	context.Context,
	...libhead.HeadOption[*header.ExtendedHeader],
) (*header.ExtendedHeader, error) {
	return s.head, nil
}

func TestGetRangeByHeight_Validation(t *testing.T) {
	from := &header.ExtendedHeader{
		RawHeader: header.RawHeader{Height: 100},
	}

	serv := Service{}

	t.Run("rejects range exceeding MaxRangeRequestSize", func(t *testing.T) {
		// request 65 headers: from.Height()+1=101 to 166 exclusive = 65 headers
		to := from.Height() + 1 + libhead.MaxRangeRequestSize + 1
		_, err := serv.GetRangeByHeight(context.Background(), from, to)
		assert.ErrorIs(t, err, libhead.ErrHeadersLimitExceeded)
	})

	t.Run("accepts range at MaxRangeRequestSize", func(t *testing.T) {
		// request exactly 64 headers: from.Height()+1=101 to 165 exclusive = 64 headers
		// this will pass the guard but panic on nil store, proving the guard accepted it.
		to := from.Height() + 1 + libhead.MaxRangeRequestSize
		assert.Panics(t, func() {
			serv.GetRangeByHeight(context.Background(), from, to) //nolint:errcheck
		})
	})

	t.Run("rejects nil 'from' header", func(t *testing.T) {
		assert.NotPanics(t, func() {
			_, err := serv.GetRangeByHeight(context.Background(), nil, 100)
			assert.ErrorContains(t, err, "'from' header is nil")
		})
	})

	t.Run("rejects reversed range", func(t *testing.T) {
		_, err := serv.GetRangeByHeight(context.Background(), from, from.Height()-1)
		assert.ErrorIs(t, err, libhead.ErrRangeMixUp)
	})

	t.Run("rejects empty range", func(t *testing.T) {
		_, err := serv.GetRangeByHeight(context.Background(), from, from.Height()+1)
		assert.ErrorIs(t, err, libhead.ErrEmptyRange)
	})
}
