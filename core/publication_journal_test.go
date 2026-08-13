package core

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/cometbft/cometbft/types"
	"github.com/ipfs/go-datastore"
	"github.com/ipfs/go-datastore/failstore"
	dsbadger "github.com/ipfs/go-ds-badger4"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/stretchr/testify/require"

	libshare "github.com/celestiaorg/go-square/v4/share"
	"github.com/celestiaorg/rsmt2d"

	"github.com/celestiaorg/celestia-node/header"
	"github.com/celestiaorg/celestia-node/header/headertest"
	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/celestia-node/share/eds/edstest"
	"github.com/celestiaorg/celestia-node/share/shwap/p2p/shrex/shrexsub"
	"github.com/celestiaorg/celestia-node/store"
)

type headerBroadcastFn func(context.Context, *header.ExtendedHeader, ...pubsub.PubOpt) error

const syncOperation = "sync"

func (fn headerBroadcastFn) Broadcast(
	ctx context.Context,
	header *header.ExtendedHeader,
	opts ...pubsub.PubOpt,
) error {
	return fn(ctx, header, opts...)
}

func TestListenerJournalsBeforeStoringEDS(t *testing.T) {
	ctx := t.Context()
	eh := headertest.RandExtendedHeader(t)

	errSync := errors.New("sync failed")
	ds := failstore.NewFailstore(datastore.NewMapDatastore(), func(op string) error {
		if op == syncOperation {
			return errSync
		}
		return nil
	})
	cl, b := listenerForHeader(t, eh, newPublicationJournal(ds))

	err := cl.handleNewSignedBlock(ctx, BlockEvent{}, b, false)
	require.ErrorIs(t, err, errSync)
	has, err := cl.store.HasByHeight(ctx, eh.Height())
	require.NoError(t, err)
	require.False(t, has)
}

func TestListenerReplaysPendingPublication(t *testing.T) {
	ctx := t.Context()
	eh := headertest.RandExtendedHeader(t)
	var syncs int
	ds := failstore.NewFailstore(datastore.NewMapDatastore(), func(op string) error {
		if op == syncOperation {
			syncs++
		}
		return nil
	})
	journal := newPublicationJournal(ds)
	require.NoError(t, journal.put(ctx, eh))

	var hashCalls int
	var headerCalls int
	cl, _ := listenerForHeader(t, eh, newPublicationJournal(ds))
	cl.hashBroadcaster = func(_ context.Context, notification shrexsub.Notification) error {
		require.Equal(t, eh.Height(), notification.Height)
		require.Equal(t, share.DataHash(eh.DataHash.Bytes()), notification.DataHash)
		hashCalls++
		if hashCalls == 1 {
			return errors.New("hash broadcast failed")
		}
		return nil
	}
	cl.headerBroadcaster = headerBroadcastFn(func(
		_ context.Context,
		got *header.ExtendedHeader,
		_ ...pubsub.PubOpt,
	) error {
		require.Equal(t, eh.Hash(), got.Hash())
		headerCalls++
		if headerCalls == 2 {
			return errors.New("header broadcast failed")
		}
		return nil
	})

	cl.retryPendingPublications(ctx)
	require.Zero(t, hashCalls)
	require.Zero(t, headerCalls)
	require.Len(t, requirePending(t, ctx, journal), 1)

	require.NoError(t, cl.store.PutODS(ctx, eh.DAH, eh.Height(), share.EmptyEDS()))

	cl.retryPendingPublications(ctx)
	require.Equal(t, 1, hashCalls)
	require.Equal(t, 1, headerCalls)
	require.Len(t, requirePending(t, ctx, journal), 1)

	cl.retryPendingPublications(ctx)
	require.Equal(t, 2, hashCalls)
	require.Equal(t, 2, headerCalls)
	require.Len(t, requirePending(t, ctx, journal), 1)

	cl.retryPendingPublications(ctx)
	require.Equal(t, 3, hashCalls)
	require.Equal(t, 3, headerCalls)
	require.Empty(t, requirePending(t, ctx, journal))
	require.Equal(t, 1, syncs)
}

func TestListenerDoesNotJournalSyncingBlock(t *testing.T) {
	ctx := t.Context()
	eh := headertest.RandExtendedHeader(t)
	var touched bool
	ds := failstore.NewFailstore(datastore.NewMapDatastore(), func(op string) error {
		if op == "put" || op == syncOperation {
			touched = true
			return errors.New("journal should not be used")
		}
		return nil
	})
	journal := newPublicationJournal(ds)
	cl, b := listenerForHeader(t, eh, journal)

	require.NoError(t, cl.handleNewSignedBlock(ctx, BlockEvent{}, b, true))
	require.Empty(t, requirePending(t, ctx, journal))
	require.False(t, touched)
}

func TestListenerJournalsFailedLiveBroadcast(t *testing.T) {
	for _, test := range []struct {
		name       string
		broadcast  error
		journalLen int
	}{
		{name: "success"},
		{name: "failure", broadcast: errors.New("broadcast failed"), journalLen: 1},
	} {
		t.Run(test.name, func(t *testing.T) {
			ctx := t.Context()
			eh := headertest.RandExtendedHeader(t)
			journal := newPublicationJournal(datastore.NewMapDatastore())
			cl, b := listenerForHeader(t, eh, journal)
			cl.hashBroadcaster = func(context.Context, shrexsub.Notification) error {
				return test.broadcast
			}

			require.NoError(t, cl.handleNewSignedBlock(ctx, BlockEvent{}, b, false))
			require.Len(t, requirePending(t, ctx, journal), test.journalLen)
		})
	}
}

func TestListenerPublicationRetryAdvancesAfterTimeout(t *testing.T) {
	ctx := t.Context()
	headers := headertest.NewTestSuite(t).GenExtendedHeaders(2)
	journal := newPublicationJournal(datastore.NewMapDatastore())
	for _, eh := range headers {
		require.NoError(t, journal.put(ctx, eh))
	}

	cl, _ := listenerForHeader(t, headers[0], journal)
	for _, eh := range headers {
		require.NoError(t, cl.store.PutODS(ctx, eh.DAH, eh.Height(), share.EmptyEDS()))
	}
	var secondCalls int
	cl.hashBroadcaster = func(ctx context.Context, notification shrexsub.Notification) error {
		if notification.Height == headers[0].Height() {
			<-ctx.Done()
			return ctx.Err()
		}
		secondCalls++
		return nil
	}

	firstCtx, cancel := context.WithTimeout(ctx, 100*time.Millisecond)
	cl.retryPendingPublications(firstCtx)
	cancel()
	require.Zero(t, secondCalls)

	secondCtx, cancel := context.WithTimeout(ctx, 100*time.Millisecond)
	cl.retryPendingPublications(secondCtx)
	cancel()
	require.Equal(t, 1, secondCalls)
}

func TestListenerPublicationRetryRevisitsFailureWithNewBacklog(t *testing.T) {
	ctx := t.Context()
	headers := headertest.NewTestSuite(t).GenExtendedHeaders(publicationRetryBatchSize * 2)
	journal := newPublicationJournal(datastore.NewMapDatastore())
	cl, _ := listenerForHeader(t, headers[0], journal)

	addPublications := func(headers []*header.ExtendedHeader) {
		for _, eh := range headers {
			require.NoError(t, journal.put(ctx, eh))
			require.NoError(t, cl.store.PutODS(ctx, eh.DAH, eh.Height(), share.EmptyEDS()))
		}
	}

	failedHeight := headers[0].Height()
	failedCalls := 0
	cl.hashBroadcaster = func(_ context.Context, notification shrexsub.Notification) error {
		if notification.Height == failedHeight {
			failedCalls++
			return errors.New("broadcast failed")
		}
		return nil
	}

	addPublications(headers[:publicationRetryBatchSize])
	cl.retryPendingPublications(ctx)
	require.Equal(t, 1, failedCalls)

	addPublications(headers[publicationRetryBatchSize:])
	cl.retryPendingPublications(ctx)
	require.Equal(t, 2, failedCalls)
}

func TestListenerPublicationRetryWorkerStops(t *testing.T) {
	cl := &Listener{
		fetcher: &fakeSource{subFn: func(int) (chan BlockEvent, error) {
			return neverDelivers(), nil
		}},
		publications:    newPublicationJournal(datastore.NewMapDatastore()),
		listenerTimeout: time.Hour,
	}
	require.NoError(t, cl.Start(t.Context()))

	ctx, cancel := context.WithTimeout(t.Context(), time.Second)
	defer cancel()
	require.NoError(t, cl.Stop(ctx))
}

func TestListenerRemovesExpiredPublication(t *testing.T) {
	t.Setenv("CELESTIA_OVERRIDE_AVAILABILITY_WINDOW", "-1s")
	ctx := t.Context()
	eh := headertest.RandExtendedHeader(t)
	journal := newPublicationJournal(datastore.NewMapDatastore())
	require.NoError(t, journal.put(ctx, eh))

	cl, _ := listenerForHeader(t, eh, journal)
	cl.archival = true
	require.NoError(t, cl.store.PutODS(ctx, eh.DAH, eh.Height(), share.EmptyEDS()))
	cl.retryPendingPublications(ctx)

	require.Empty(t, requirePending(t, ctx, journal))
}

func TestListenerRemovesPublicationWithMismatchedEDS(t *testing.T) {
	ctx := t.Context()
	eh := headertest.RandExtendedHeader(t)
	journal := newPublicationJournal(datastore.NewMapDatastore())
	require.NoError(t, journal.put(ctx, eh))

	cl, _ := listenerForHeader(t, eh, journal)
	eds := edstest.RandEDS(t, 4)
	roots, err := share.NewAxisRoots(eds)
	require.NoError(t, err)
	require.NoError(t, cl.store.PutODS(ctx, roots, eh.Height(), eds))
	cl.hashBroadcaster = func(context.Context, shrexsub.Notification) error {
		t.Fatal("broadcasted a mismatched data hash")
		return nil
	}
	cl.headerBroadcaster = headerBroadcastFn(func(
		context.Context,
		*header.ExtendedHeader,
		...pubsub.PubOpt,
	) error {
		t.Fatal("broadcasted a header for a mismatched EDS")
		return nil
	})

	cl.retryPendingPublications(ctx)

	require.Empty(t, requirePending(t, ctx, journal))
}

func TestListenerRemovesWrongChainPublication(t *testing.T) {
	ctx := t.Context()
	eh := headertest.RandExtendedHeader(t)
	journal := newPublicationJournal(datastore.NewMapDatastore())
	require.NoError(t, journal.put(ctx, eh))

	cl, _ := listenerForHeader(t, eh, journal)
	cl.chainID = eh.ChainID() + "-other"
	cl.hashBroadcaster = func(context.Context, shrexsub.Notification) error {
		t.Fatal("broadcasted a data hash for the wrong chain")
		return nil
	}
	cl.headerBroadcaster = headerBroadcastFn(func(
		context.Context,
		*header.ExtendedHeader,
		...pubsub.PubOpt,
	) error {
		t.Fatal("broadcasted a header for the wrong chain")
		return nil
	})

	cl.retryPendingPublications(ctx)

	require.Empty(t, requirePending(t, ctx, journal))
}

func TestPublicationJournalValidatesRecord(t *testing.T) {
	ctx := t.Context()
	eh := headertest.RandExtendedHeader(t)
	journal := newPublicationJournal(datastore.NewMapDatastore())
	require.NoError(t, journal.put(ctx, eh))

	key := publicationKey(eh.Height())
	record, err := journal.Get(ctx, key)
	require.NoError(t, err)
	require.NoError(t, journal.Delete(ctx, key))

	wrongKey := publicationKey(eh.Height() + 1)
	require.NoError(t, journal.Put(ctx, wrongKey, record))
	require.Empty(t, requirePending(t, ctx, journal))
	has, err := journal.Has(ctx, wrongKey)
	require.NoError(t, err)
	require.False(t, has)

	invalid := *eh
	invalid.DataHash = make([]byte, len(eh.DataHash))
	headerData, err := invalid.MarshalBinary()
	require.NoError(t, err)
	_, err = decodePendingPublication(key.String(), headerData)
	require.ErrorContains(t, err, "validating header")
}

func TestPublicationJournalPaginates(t *testing.T) {
	ctx := t.Context()
	ds := openPublicationBadger(t, t.TempDir())
	t.Cleanup(func() { require.NoError(t, ds.Close()) })
	journal := newPublicationJournal(ds)
	headers := headertest.NewTestSuite(t).GenExtendedHeaders(publicationRetryBatchSize + 1)
	for _, eh := range headers {
		require.NoError(t, journal.put(ctx, eh))
	}

	first, cursor, err := journal.pending(ctx, "", "")
	require.NoError(t, err)
	require.Len(t, first, publicationRetryBatchSize)
	require.NotEmpty(t, cursor)

	second, cursor, err := journal.pending(ctx, cursor, "")
	require.NoError(t, err)
	require.Len(t, second, 1)
	require.NotEmpty(t, cursor)

	wrapped, _, err := journal.pending(ctx, cursor, "")
	require.NoError(t, err)
	require.Empty(t, wrapped)
}

func TestPublicationJournalConcurrentAccess(t *testing.T) {
	const workers = 8
	ctx := t.Context()
	eh := headertest.RandExtendedHeader(t)
	journal := newPublicationJournal(datastore.NewMapDatastore())
	start := make(chan struct{})
	done := make(chan error, workers)
	for range workers {
		go func() {
			<-start
			for range 10 {
				if err := journal.put(ctx, eh); err != nil {
					done <- err
					return
				}
				if _, _, err := journal.pending(ctx, "", ""); err != nil {
					done <- err
					return
				}
			}
			done <- nil
		}()
	}
	close(start)
	for range workers {
		require.NoError(t, <-done)
	}
}

func TestPublicationJournalSurvivesBadgerRestart(t *testing.T) {
	ctx := t.Context()
	eh := headertest.RandExtendedHeader(t)
	path := t.TempDir()

	ds := openPublicationBadger(t, path)
	require.NoError(t, newPublicationJournal(ds).put(ctx, eh))
	require.NoError(t, ds.Close())

	ds = openPublicationBadger(t, path)
	t.Cleanup(func() { require.NoError(t, ds.Close()) })
	publications := requirePending(t, ctx, newPublicationJournal(ds))
	require.Len(t, publications, 1)
	require.True(t, eh.Equals(publications[0]))
}

func listenerForHeader(
	t *testing.T,
	eh *header.ExtendedHeader,
	journal *publicationJournal,
) (*Listener, *SignedBlock) {
	t.Helper()
	ctx := t.Context()
	st, err := store.NewStore(store.DefaultParameters(), t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, st.Stop(ctx)) })

	cl := &Listener{
		construct: func(
			*types.Header,
			*types.Commit,
			*types.ValidatorSet,
			*rsmt2d.ExtendedDataSquare,
		) (*header.ExtendedHeader, error) {
			return eh, nil
		},
		store:              st,
		availabilityWindow: time.Hour,
		publications:       journal,
		hashBroadcaster: func(context.Context, shrexsub.Notification) error {
			return nil
		},
		headerBroadcaster: headerBroadcastFn(func(
			context.Context,
			*header.ExtendedHeader,
			...pubsub.PubOpt,
		) error {
			return nil
		}),
	}
	b := &SignedBlock{
		Header:       &eh.RawHeader,
		Commit:       eh.Commit,
		Data:         &types.Data{},
		ValidatorSet: eh.ValidatorSet,
	}
	return cl, b
}

func openPublicationBadger(t testing.TB, path string) *dsbadger.Datastore {
	t.Helper()
	options := dsbadger.DefaultOptions
	options.GcInterval = 0
	options.SyncWrites = false
	options.ValueThreshold = libshare.ShareSize
	ds, err := dsbadger.NewDatastore(path, &options)
	require.NoError(t, err)
	return ds
}

func requirePending(
	t *testing.T,
	ctx context.Context,
	journal *publicationJournal,
) []*header.ExtendedHeader {
	t.Helper()
	publications, _, err := journal.pending(ctx, "", "")
	require.NoError(t, err)
	return publications
}
