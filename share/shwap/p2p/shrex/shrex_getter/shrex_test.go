package shrex_getter //nolint:stylecheck // underscore in pkg name will be fixed with shrex refactoring

import (
	"bytes"
	"context"
	"fmt"
	"math/rand/v2"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ipfs/go-datastore"
	ds_sync "github.com/ipfs/go-datastore/sync"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/p2p/net/conngater"
	mocknet "github.com/libp2p/go-libp2p/p2p/net/mock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/celestiaorg/celestia-app/v7/pkg/appconsts"
	"github.com/celestiaorg/celestia-app/v7/pkg/wrapper"
	libhead "github.com/celestiaorg/go-header"
	"github.com/celestiaorg/go-square/merkle"
	"github.com/celestiaorg/go-square/v3/inclusion"
	libshare "github.com/celestiaorg/go-square/v3/share"
	"github.com/celestiaorg/nmt"
	"github.com/celestiaorg/rsmt2d"

	"github.com/celestiaorg/celestia-node/header"
	"github.com/celestiaorg/celestia-node/header/headertest"
	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/celestia-node/share/availability"
	"github.com/celestiaorg/celestia-node/share/eds"
	"github.com/celestiaorg/celestia-node/share/eds/edstest"
	"github.com/celestiaorg/celestia-node/share/shwap"
	"github.com/celestiaorg/celestia-node/share/shwap/p2p/shrex"
	"github.com/celestiaorg/celestia-node/share/shwap/p2p/shrex/peers"
	"github.com/celestiaorg/celestia-node/share/shwap/p2p/shrex/shrexsub"
	"github.com/celestiaorg/celestia-node/store"
)

func TestShrexGetter(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	t.Cleanup(cancel)

	// create test net
	net, err := mocknet.FullMeshConnected(2)
	require.NoError(t, err)
	clHost, srvHost := net.Hosts()[0], net.Hosts()[1]

	// launch eds store and put test data into it

	st, err := newStore(t)
	require.NoError(t, err)
	edsStore := newWrappedStore(st)
	client, _ := newShrexClientServer(ctx, t, edsStore, srvHost, clHost)

	// create shrex Getter
	sub := new(headertest.Subscriber)

	fullPeerManager, err := testManager(ctx, clHost, sub)
	require.NoError(t, err)
	archivalPeerManager, err := testManager(ctx, clHost, sub)
	require.NoError(t, err)

	getter := NewGetter(client, fullPeerManager, archivalPeerManager, availability.RequestWindow)
	require.NoError(t, getter.Start(ctx))

	height := atomic.Uint64{}
	height.Add(1)

	t.Run("ND_Available, total data size > 1mb", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(ctx, time.Second*10)
		t.Cleanup(cancel)

		// generate test data
		size := 64
		namespace := libshare.RandomNamespace()
		height := height.Add(1)
		randEDS, roots := edstest.RandEDSWithNamespace(t, namespace, size*size, size)
		eh := headertest.RandExtendedHeaderWithRoot(t, roots)
		eh.RawHeader.Height = int64(height)

		err = edsStore.PutODSQ4(ctx, roots, height, randEDS)
		require.NoError(t, err)
		fullPeerManager.Validate(ctx, srvHost.ID(), shrexsub.Notification{
			DataHash: roots.Hash(),
			Height:   height,
		})

		got, err := getter.GetNamespaceData(ctx, eh, namespace)
		require.NoError(t, err)
		require.NoError(t, got.Verify(roots, namespace))
	})

	t.Run("ND_err_not_found", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(ctx, time.Second)
		t.Cleanup(cancel)

		// generate test data
		height := height.Add(1)
		_, roots, namespace := generateTestEDS(t)
		eh := headertest.RandExtendedHeaderWithRoot(t, roots)
		eh.RawHeader.Height = int64(height)

		fullPeerManager.Validate(ctx, srvHost.ID(), shrexsub.Notification{
			DataHash: roots.Hash(),
			Height:   height,
		})

		_, err := getter.GetNamespaceData(ctx, eh, namespace)
		require.ErrorIs(t, err, shwap.ErrNotFound)
	})

	t.Run("ND_namespace_not_included", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(ctx, time.Second)
		t.Cleanup(cancel)

		// generate test data
		height := height.Add(1)
		eds, roots, maxNamespace := generateTestEDS(t)
		eh := headertest.RandExtendedHeaderWithRoot(t, roots)
		eh.RawHeader.Height = int64(height)

		err = edsStore.PutODSQ4(ctx, roots, height, eds)
		require.NoError(t, err)
		fullPeerManager.Validate(ctx, srvHost.ID(), shrexsub.Notification{
			DataHash: roots.Hash(),
			Height:   height,
		})

		// namespace inside root range
		nID, err := maxNamespace.AddInt(-1)
		require.NoError(t, err)
		// check for namespace to be between max and min namespace in root
		rows, err := share.RowsWithNamespace(roots, nID)
		require.NoError(t, err)
		require.Len(t, rows, 1)

		emptyShares, err := getter.GetNamespaceData(ctx, eh, nID)
		require.NoError(t, err)
		// no shares should be returned
		require.Nil(t, emptyShares.Flatten())
		require.Nil(t, emptyShares.Verify(roots, nID))

		// namespace outside root range
		nID, err = maxNamespace.AddInt(1)
		require.NoError(t, err)
		// check for namespace to be not in root
		rows, err = share.RowsWithNamespace(roots, nID)
		require.NoError(t, err)
		require.Len(t, rows, 0)

		emptyShares, err = getter.GetNamespaceData(ctx, eh, nID)
		require.NoError(t, err)
		// no shares should be returned
		require.Nil(t, emptyShares.Flatten())
		require.Nil(t, emptyShares.Verify(roots, nID))
	})

	t.Run("ND_namespace_not_in_dah", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(ctx, time.Second)
		t.Cleanup(cancel)

		// generate test data
		eds, roots, maxNamespace := generateTestEDS(t)
		height := height.Add(1)
		eh := headertest.RandExtendedHeaderWithRoot(t, roots)
		eh.RawHeader.Height = int64(height)

		err = edsStore.PutODSQ4(ctx, roots, height, eds)
		require.NoError(t, err)
		fullPeerManager.Validate(ctx, srvHost.ID(), shrexsub.Notification{
			DataHash: roots.Hash(),
			Height:   height,
		})

		namespace, err := maxNamespace.AddInt(1)
		require.NoError(t, err)
		// check for namespace to be not in root
		rows, err := share.RowsWithNamespace(roots, namespace)
		require.NoError(t, err)
		require.Len(t, rows, 0)

		emptyShares, err := getter.GetNamespaceData(ctx, eh, namespace)
		require.NoError(t, err)
		// no shares should be returned
		require.Empty(t, emptyShares.Flatten())
		require.Nil(t, emptyShares.Verify(roots, namespace))
	})

	t.Run("EDS_Available", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(ctx, time.Second)
		t.Cleanup(cancel)

		// generate test data
		randEDS, roots, _ := generateTestEDS(t)
		height := height.Add(1)
		eh := headertest.RandExtendedHeaderWithRoot(t, roots)
		eh.RawHeader.Height = int64(height)

		err = edsStore.PutODSQ4(ctx, roots, height, randEDS)
		require.NoError(t, err)
		fullPeerManager.Validate(ctx, srvHost.ID(), shrexsub.Notification{
			DataHash: roots.Hash(),
			Height:   height,
		})

		got, err := getter.GetEDS(ctx, eh)
		require.NoError(t, err)
		require.Equal(t, randEDS.Flattened(), got.Flattened())
	})

	t.Run("EDS_ctx_deadline", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(ctx, time.Second)

		// generate test data
		_, roots, _ := generateTestEDS(t)
		height := height.Add(1)
		eh := headertest.RandExtendedHeaderWithRoot(t, roots)
		eh.RawHeader.Height = int64(height)

		fullPeerManager.Validate(ctx, srvHost.ID(), shrexsub.Notification{
			DataHash: roots.Hash(),
			Height:   height,
		})

		cancel()
		_, err := getter.GetEDS(ctx, eh)
		require.ErrorIs(t, err, context.Canceled)
	})

	t.Run("EDS_err_not_found", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(ctx, time.Second)
		t.Cleanup(cancel)

		// generate test data
		_, roots, _ := generateTestEDS(t)
		height := height.Add(1)
		eh := headertest.RandExtendedHeaderWithRoot(t, roots)
		eh.RawHeader.Height = int64(height)

		fullPeerManager.Validate(ctx, srvHost.ID(), shrexsub.Notification{
			DataHash: roots.Hash(),
			Height:   height,
		})

		_, err := getter.GetEDS(ctx, eh)
		require.ErrorIs(t, err, shwap.ErrNotFound)
	})

	t.Run("Samples_Available", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(ctx, time.Second)
		t.Cleanup(cancel)

		// generate test data
		randEDS, roots, _ := generateTestEDS(t)
		height := height.Add(1)
		eh := headertest.RandExtendedHeaderWithRoot(t, roots)
		eh.RawHeader.Height = int64(height)

		err = edsStore.PutODSQ4(ctx, roots, height, randEDS)
		require.NoError(t, err)
		fullPeerManager.Validate(ctx, srvHost.ID(), shrexsub.Notification{
			DataHash: roots.Hash(),
			Height:   height,
		})

		odsSize := len(eh.DAH.RowRoots) / 2
		coords := make([]shwap.SampleCoords, odsSize*odsSize)
		total := 0
		for i := 0; i < odsSize; i++ {
			for j := 0; j < odsSize; j++ {
				coords[total] = shwap.SampleCoords{Row: i, Col: j}
				total++
			}
		}

		got, err := getter.GetSamples(ctx, eh, coords)
		require.NoError(t, err)
		assert.Len(t, got, len(coords))

		odsShares := randEDS.FlattenedODS()
		require.NoError(t, err)
		for i, samples := range got {
			assert.Equal(t, odsShares[i], samples.ToBytes(), fmt.Sprintf("sample %d", i))
		}
	})

	t.Run("Samples_Failed_coords_out_of_bound", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(ctx, time.Second)
		t.Cleanup(cancel)

		// generate test data
		randEDS, roots, _ := generateTestEDS(t)
		height := height.Add(1)
		eh := headertest.RandExtendedHeaderWithRoot(t, roots)
		eh.RawHeader.Height = int64(height)

		err = edsStore.PutODSQ4(ctx, roots, height, randEDS)
		require.NoError(t, err)
		fullPeerManager.Validate(ctx, srvHost.ID(), shrexsub.Notification{
			DataHash: roots.Hash(),
			Height:   height,
		})

		coords := []shwap.SampleCoords{
			{Row: 0, Col: 10},
		}

		_, err := getter.GetSamples(ctx, eh, coords)
		require.Error(t, err)
		assert.ErrorIs(t, err, shwap.ErrOutOfBounds)
	})

	t.Run("Samples_partial_response", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(ctx, time.Second)
		t.Cleanup(cancel)

		// generate test data
		randEDS, roots, _ := generateTestEDS(t)
		height := height.Add(1)
		eh := headertest.RandExtendedHeaderWithRoot(t, roots)
		eh.RawHeader.Height = int64(height)

		err = edsStore.PutODSQ4(ctx, roots, height, randEDS)
		require.NoError(t, err)
		fullPeerManager.Validate(ctx, srvHost.ID(), shrexsub.Notification{
			DataHash: roots.Hash(),
			Height:   height,
		})
		edsStore.targetHeight = int64(height)
		edsStore.targetCoords = shwap.SampleCoords{Row: 0, Col: 5}
		coords := []shwap.SampleCoords{
			{Row: 0, Col: 1},
			{Row: 0, Col: 2},
			{Row: 0, Col: 5},
			{Row: 1, Col: 0},
		}

		samples, err := getter.GetSamples(ctx, eh, coords)
		require.Error(t, err)
		assert.NotNil(t, samples)
		assert.Len(t, samples, 3)
	})

	t.Run("Samples_EmptyBlock", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(ctx, time.Second)
		t.Cleanup(cancel)

		eh := headertest.RandExtendedHeaderWithRoot(t, share.EmptyEDSRoots())

		coords := []shwap.SampleCoords{
			{Row: 0, Col: 0},
			{Row: 0, Col: 1},
			{Row: 1, Col: 0},
		}

		got, err := getter.GetSamples(ctx, eh, coords)
		require.NoError(t, err)
		require.Len(t, got, len(coords))
		for _, smpl := range got {
			require.False(t, smpl.IsEmpty(), "sample should not be empty for empty block")
			require.NotNil(t, smpl.Proof, "sample proof should not be nil for empty block")
		}
	})

	t.Run("Samples_EmptyBlock_OutOfBounds", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(ctx, time.Second)
		t.Cleanup(cancel)

		eh := headertest.RandExtendedHeaderWithRoot(t, share.EmptyEDSRoots())

		coords := []shwap.SampleCoords{
			{Row: 0, Col: 5},
		}

		_, err := getter.GetSamples(ctx, eh, coords)
		require.Error(t, err)
		assert.ErrorIs(t, err, shwap.ErrOutOfBounds)
	})

	t.Run("Row_Available", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(ctx, time.Second)
		t.Cleanup(cancel)

		// generate test data
		randEDS, roots, _ := generateTestEDS(t)
		height := height.Add(1)
		eh := headertest.RandExtendedHeaderWithRoot(t, roots)
		eh.RawHeader.Height = int64(height)

		err = edsStore.PutODSQ4(ctx, roots, height, randEDS)
		require.NoError(t, err)
		fullPeerManager.Validate(ctx, srvHost.ID(), shrexsub.Notification{
			DataHash: roots.Hash(),
			Height:   height,
		})

		odsSize := len(roots.RowRoots)
		for i := 0; i < odsSize; i++ {
			row, err := getter.GetRow(ctx, eh, i)
			require.NoError(t, err)
			shrs, err := row.Shares()
			require.NoError(t, err)
			edsRow := randEDS.Row(uint(i))
			for j, shr := range shrs {
				require.Equal(t, edsRow[j], shr.ToBytes())
			}
		}
	})

	t.Run("Row_Failed_row_index_out_of_bound", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(ctx, time.Second)
		t.Cleanup(cancel)

		// generate test data
		randEDS, roots, _ := generateTestEDS(t)
		height := height.Add(1)
		eh := headertest.RandExtendedHeaderWithRoot(t, roots)
		eh.RawHeader.Height = int64(height)

		err = edsStore.PutODSQ4(ctx, roots, height, randEDS)
		require.NoError(t, err)
		fullPeerManager.Validate(ctx, srvHost.ID(), shrexsub.Notification{
			DataHash: roots.Hash(),
			Height:   height,
		})

		_, err := getter.GetRow(ctx, eh, 20)
		require.Error(t, err)
		assert.ErrorIs(t, err, shwap.ErrOutOfBounds)
	})

	// tests getPeer's ability to route requests based on whether
	// they are historical or not
	t.Run("routing_historical_requests", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(ctx, time.Second)
		t.Cleanup(cancel)

		archivalPeer, err := net.GenPeer()
		require.NoError(t, err)
		fullPeer, err := net.GenPeer()
		require.NoError(t, err)

		getter.archivalPeerManager.UpdateNodePool(archivalPeer.ID(), true)
		getter.fullPeerManager.UpdateNodePool(fullPeer.ID(), true)

		height := height.Add(1)
		eh := headertest.RandExtendedHeader(t)
		eh.RawHeader.Height = int64(height)

		// historical data expects an archival peer
		eh.RawHeader.Time = time.Now().Add(-(availability.StorageWindow + time.Second))
		id, _, err := getter.getPeer(ctx, eh)
		require.NoError(t, err)
		assert.Equal(t, archivalPeer.ID(), id)

		// recent (within sampling window) data expects a full peer
		eh.RawHeader.Time = time.Now()
		id, _, err = getter.getPeer(ctx, eh)
		require.NoError(t, err)
		assert.Equal(t, fullPeer.ID(), id)
	})

	t.Run("GetRangeNamespaceData", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(ctx, time.Second*2)
		t.Cleanup(cancel)

		// generate test data
		size := 64
		namespace := libshare.RandomNamespace()
		height := height.Add(1)
		randEDS, roots := edstest.RandEDSWithNamespace(t, namespace, size*size, size)
		eh := headertest.RandExtendedHeaderWithRoot(t, roots)
		eh.RawHeader.Height = int64(height)

		err = edsStore.PutODSQ4(ctx, roots, height, randEDS)
		require.NoError(t, err)
		fullPeerManager.Validate(ctx, srvHost.ID(), shrexsub.Notification{
			DataHash: roots.Hash(),
			Height:   height,
		})
		_, err = getter.GetNamespaceData(ctx, eh, namespace)
		require.NoError(t, err)

		odsSize := len(roots.RowRoots) / 2

		sharesAmount := odsSize * odsSize
		fromIndex := rand.IntN(odsSize)
		inclusiveToIndex := sharesAmount - fromIndex

		from, err := shwap.SampleCoordsFrom1DIndex(fromIndex, odsSize)
		require.NoError(t, err)
		to, err := shwap.SampleCoordsFrom1DIndex(inclusiveToIndex, odsSize)
		require.NoError(t, err)
		rngdata, err := getter.GetRangeNamespaceData(context.Background(), eh, fromIndex, inclusiveToIndex+1)
		require.NoError(t, err)
		err = rngdata.VerifyInclusion(
			from, to,
			len(eh.DAH.RowRoots)/2,
			eh.DAH.RowRoots[from.Row:to.Row+1],
		)
		require.NoError(t, err)
		t.Logf("range of %d shares", inclusiveToIndex-fromIndex+1)
	})
}

func newStore(t *testing.T) (*store.Store, error) {
	t.Helper()

	return store.NewStore(store.DefaultParameters(), t.TempDir())
}

func generateTestEDS(t *testing.T) (*rsmt2d.ExtendedDataSquare, *share.AxisRoots, libshare.Namespace) {
	eds := edstest.RandEDS(t, 4)
	roots, err := share.NewAxisRoots(eds)
	require.NoError(t, err)
	max := nmt.MaxNamespace(roots.RowRoots[(len(roots.RowRoots))/2-1], libshare.NamespaceSize)
	ns, err := libshare.NewNamespaceFromBytes(max)
	require.NoError(t, err)
	return eds, roots, ns
}

func testManager(
	ctx context.Context, host host.Host, headerSub libhead.Subscriber[*header.ExtendedHeader],
) (*peers.Manager, error) {
	shrexSub, err := shrexsub.NewPubSub(ctx, host, "test")
	if err != nil {
		return nil, err
	}

	connGater, err := conngater.NewBasicConnectionGater(ds_sync.MutexWrap(datastore.NewMapDatastore()))
	if err != nil {
		return nil, err
	}
	manager, err := peers.NewManager(
		*peers.DefaultParameters(),
		host,
		connGater,
		"test",
		peers.WithShrexSubPools(shrexSub, headerSub),
	)
	return manager, err
}

func newShrexClientServer(
	ctx context.Context, t *testing.T, edsStore store.AccessorGetter, srvHost, clHost host.Host,
) (*shrex.Client, *shrex.Server) {
	// create server and register handler
	server, err := shrex.NewServer(shrex.DefaultServerParameters(), srvHost, edsStore)
	require.NoError(t, err)
	require.NoError(t, server.WithMetrics())
	require.NoError(t, server.Start(ctx))
	t.Cleanup(func() {
		_ = server.Stop(ctx)
	})

	// create client and connect it to server
	client, err := shrex.NewClient(shrex.DefaultClientParameters(), clHost)
	require.NoError(t, err)
	require.NoError(t, client.WithMetrics())
	return client, server
}

type wrappedStore struct {
	*store.Store

	targetCoords shwap.SampleCoords
	targetHeight int64
}

func newWrappedStore(store *store.Store) *wrappedStore {
	return &wrappedStore{
		Store:        store,
		targetHeight: -1,
	}
}

func (w *wrappedStore) GetByHeight(ctx context.Context, height uint64) (eds.AccessorStreamer, error) {
	accessorStreamer, err := w.Store.GetByHeight(ctx, height)
	if err != nil {
		return nil, err
	}
	if int64(height) != w.targetHeight {
		return accessorStreamer, nil
	}
	return wrappedAccessorStreamer{AccessorStreamer: accessorStreamer, targetCoords: w.targetCoords}, nil
}

type wrappedAccessorStreamer struct {
	eds.AccessorStreamer

	targetCoords shwap.SampleCoords
}

func (w wrappedAccessorStreamer) Sample(ctx context.Context, coords shwap.SampleCoords) (shwap.Sample, error) {
	if w.targetCoords.Row == coords.Row && w.targetCoords.Col == coords.Col {
		return shwap.Sample{}, shwap.ErrNotFound
	}

	return w.AccessorStreamer.Sample(ctx, coords)
}

// setupBlobTest creates the common test infrastructure: mocknet, store, shrex client/server,
// peer managers, and getter. Returns the getter, edsStore, height counter, srvHost ID,
// and the fullPeerManager (for registering notifications).
func setupBlobTest(
	t *testing.T,
	ctx context.Context,
) (*Getter, *wrappedStore, *atomic.Uint64, *peers.Manager, host.Host) {
	t.Helper()

	net, err := mocknet.FullMeshConnected(2)
	require.NoError(t, err)
	clHost, srvHost := net.Hosts()[0], net.Hosts()[1]

	st, err := newStore(t)
	require.NoError(t, err)
	edsStore := newWrappedStore(st)
	client, _ := newShrexClientServer(ctx, t, edsStore, srvHost, clHost)

	sub := new(headertest.Subscriber)
	fullPeerManager, err := testManager(ctx, clHost, sub)
	require.NoError(t, err)
	archivalPeerManager, err := testManager(ctx, clHost, sub)
	require.NoError(t, err)

	getter := NewGetter(client, fullPeerManager, archivalPeerManager, availability.RequestWindow)
	require.NoError(t, getter.Start(ctx))

	height := &atomic.Uint64{}
	height.Add(1)

	return getter, edsStore, height, fullPeerManager, srvHost
}

// storeEDSFromBlobs creates an EDS from given blobs (already sorted by namespace),
// stores it, registers the peer notification, and returns the header.
func storeEDSFromBlobs(
	t *testing.T,
	ctx context.Context,
	blobs []*libshare.Blob,
	odsSize int,
	edsStore *wrappedStore,
	height *atomic.Uint64,
	peerManager *peers.Manager,
	srvHost host.Host,
) *header.ExtendedHeader {
	t.Helper()

	var allShares []libshare.Share
	for _, b := range blobs {
		shrs, err := b.ToShares()
		require.NoError(t, err)
		allShares = append(allShares, shrs...)
	}

	totalShares := odsSize * odsSize
	require.LessOrEqual(t, len(allShares), totalShares, "blobs don't fit in ODS")
	if len(allShares) < totalShares {
		allShares = append(allShares, libshare.TailPaddingShares(totalShares-len(allShares))...)
	}

	randEDS, err := rsmt2d.ComputeExtendedDataSquare(
		libshare.ToBytes(allShares),
		share.DefaultRSMT2DCodec(),
		wrapper.NewConstructor(uint64(odsSize)),
	)
	require.NoError(t, err)

	roots, err := share.NewAxisRoots(randEDS)
	require.NoError(t, err)

	h := height.Add(1)
	eh := headertest.RandExtendedHeaderWithRoot(t, roots)
	eh.RawHeader.Height = int64(h)

	err = edsStore.PutODSQ4(ctx, roots, h, randEDS)
	require.NoError(t, err)

	peerManager.Validate(ctx, srvHost.ID(), shrexsub.Notification{
		DataHash: roots.Hash(),
		Height:   h,
	})

	return eh
}

// computeCommitment computes the blob commitment for a libshare.Blob.
func computeCommitment(t *testing.T, blob *libshare.Blob) []byte {
	t.Helper()
	commitment, err := inclusion.CreateCommitment(blob, merkle.HashFromByteSlices, appconsts.SubtreeRootThreshold)
	require.NoError(t, err)
	return commitment
}

func TestGetBlob(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*30)
	t.Cleanup(cancel)

	getter, edsStore, height, peerManager, srvHost := setupBlobTest(t, ctx)

	t.Run("small_blob_single_share", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(ctx, time.Second*10)
		t.Cleanup(cancel)

		odsSize := 4
		ns := libshare.RandomBlobNamespace()
		data := bytes.Repeat([]byte{0x01}, 100)
		blob, err := libshare.NewV0Blob(ns, data)
		require.NoError(t, err)

		shrs, err := blob.ToShares()
		require.NoError(t, err)
		require.Len(t, shrs, 1, "blob should fit in 1 share")

		eh := storeEDSFromBlobs(t, ctx, []*libshare.Blob{blob}, odsSize, edsStore, height, peerManager, srvHost)
		commitment := computeCommitment(t, blob)

		got, err := getter.GetBlob(ctx, eh, ns, commitment)
		require.NoError(t, err)
		require.NotNil(t, got)

		err = got.Verify(eh.DAH.RowRoots, commitment)
		require.NoError(t, err)

		reconstructed, err := got.Blob()
		require.NoError(t, err)
		require.Equal(t, data, reconstructed.Data())
	})

	t.Run("blob_spanning_multiple_rows", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(ctx, time.Second*10)
		t.Cleanup(cancel)

		odsSize := 4
		ns := libshare.RandomBlobNamespace()
		// Create blob that spans 6 shares (> 4 cols per row, so it crosses row boundary)
		shareCount := 6
		dataSize := libshare.FirstSparseShareContentSize +
			(shareCount-1)*libshare.ContinuationSparseShareContentSize
		data := bytes.Repeat([]byte{0x02}, dataSize)
		blob, err := libshare.NewV0Blob(ns, data)
		require.NoError(t, err)

		shrs, err := blob.ToShares()
		require.NoError(t, err)
		require.Equal(t, shareCount, len(shrs), "blob should span exactly %d shares", shareCount)

		eh := storeEDSFromBlobs(t, ctx, []*libshare.Blob{blob}, odsSize, edsStore, height, peerManager, srvHost)
		commitment := computeCommitment(t, blob)

		got, err := getter.GetBlob(ctx, eh, ns, commitment)
		require.NoError(t, err)
		require.NotNil(t, got)

		err = got.Verify(eh.DAH.RowRoots, commitment)
		require.NoError(t, err)

		reconstructed, err := got.Blob()
		require.NoError(t, err)
		require.Equal(t, data, reconstructed.Data())
	})

	t.Run("blob_starting_mid_row", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(ctx, time.Second*10)
		t.Cleanup(cancel)

		odsSize := 4
		// Create two blobs with ordered namespaces so second one starts mid-row.
		ns1Bytes := bytes.Repeat([]byte{0x01}, libshare.NamespaceVersionZeroIDSize)
		ns1, err := libshare.NewV0Namespace(ns1Bytes)
		require.NoError(t, err)
		data1 := bytes.Repeat([]byte{0x03}, libshare.FirstSparseShareContentSize+
			libshare.ContinuationSparseShareContentSize)
		blob1, err := libshare.NewV0Blob(ns1, data1) // 2 shares
		require.NoError(t, err)

		ns2Bytes := bytes.Repeat([]byte{0x02}, libshare.NamespaceVersionZeroIDSize)
		ns2, err := libshare.NewV0Namespace(ns2Bytes)
		require.NoError(t, err)
		shareCount2 := 3
		dataSize2 := libshare.FirstSparseShareContentSize +
			(shareCount2-1)*libshare.ContinuationSparseShareContentSize
		data2 := bytes.Repeat([]byte{0x04}, dataSize2)
		blob2, err := libshare.NewV0Blob(ns2, data2) // 3 shares, starts at col 2
		require.NoError(t, err)

		// blobs already sorted by namespace (ns1 < ns2)
		eh := storeEDSFromBlobs(t, ctx, []*libshare.Blob{blob1, blob2}, odsSize,
			edsStore, height, peerManager, srvHost)

		// Fetch the second blob which starts mid-row
		commitment := computeCommitment(t, blob2)
		got, err := getter.GetBlob(ctx, eh, ns2, commitment)
		require.NoError(t, err)
		require.NotNil(t, got)

		err = got.Verify(eh.DAH.RowRoots, commitment)
		require.NoError(t, err)

		reconstructed, err := got.Blob()
		require.NoError(t, err)
		require.Equal(t, data2, reconstructed.Data())
	})

	t.Run("blob_not_found_wrong_commitment", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(ctx, time.Second*3)
		t.Cleanup(cancel)

		odsSize := 4
		ns := libshare.RandomBlobNamespace()
		data := bytes.Repeat([]byte{0x05}, 100)
		blob, err := libshare.NewV0Blob(ns, data)
		require.NoError(t, err)

		eh := storeEDSFromBlobs(t, ctx, []*libshare.Blob{blob}, odsSize, edsStore, height, peerManager, srvHost)

		wrongCommitment := bytes.Repeat([]byte{0xAA}, 32)
		_, err = getter.GetBlob(ctx, eh, ns, wrongCommitment)
		require.Error(t, err)
	})
}

func TestGetBlobs(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*30)
	t.Cleanup(cancel)

	getter, edsStore, height, peerManager, srvHost := setupBlobTest(t, ctx)

	t.Run("multiple_blobs_same_namespace", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(ctx, time.Second*10)
		t.Cleanup(cancel)

		odsSize := 8
		ns := libshare.RandomBlobNamespace()

		// Create 3 blobs with same namespace but different data
		data1 := bytes.Repeat([]byte{0x01}, 100)
		data2 := bytes.Repeat([]byte{0x02}, 200)
		data3 := bytes.Repeat([]byte{0x03}, 300)

		blob1, err := libshare.NewV0Blob(ns, data1)
		require.NoError(t, err)
		blob2, err := libshare.NewV0Blob(ns, data2)
		require.NoError(t, err)
		blob3, err := libshare.NewV0Blob(ns, data3)
		require.NoError(t, err)

		blobs := []*libshare.Blob{blob1, blob2, blob3}

		eh := storeEDSFromBlobs(t, ctx, blobs, odsSize, edsStore, height, peerManager, srvHost)

		got, err := getter.GetBlobs(ctx, eh, ns)
		require.NoError(t, err)
		require.Len(t, got, 3)

		// Verify each blob's inclusion proof
		for _, b := range got {
			err = b.VerifyInclusion(eh.DAH.RowRoots)
			require.NoError(t, err)
		}

		// Verify data matches expected blobs (returned in order)
		expectedData := [][]byte{data1, data2, data3}
		for i, b := range got {
			reconstructed, err := b.Blob()
			require.NoError(t, err)
			require.Equal(t, expectedData[i], reconstructed.Data(),
				"blob %d data mismatch", i)
		}
	})

	t.Run("namespace_not_found", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(ctx, time.Second*3)
		t.Cleanup(cancel)

		odsSize := 4
		ns := libshare.RandomBlobNamespace()
		data := bytes.Repeat([]byte{0x01}, 100)
		blob, err := libshare.NewV0Blob(ns, data)
		require.NoError(t, err)

		eh := storeEDSFromBlobs(t, ctx, []*libshare.Blob{blob}, odsSize,
			edsStore, height, peerManager, srvHost)

		// Request blobs for a namespace that doesn't exist in the EDS.
		// The server will return an internal error since no blobs match.
		differentNs := libshare.RandomBlobNamespace()
		_, err = getter.GetBlobs(ctx, eh, differentNs)
		require.Error(t, err)
	})
}
