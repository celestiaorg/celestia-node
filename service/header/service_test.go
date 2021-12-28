package header

import (
	"context"
	"testing"

	"github.com/ipfs/go-datastore"
	mdutils "github.com/ipfs/go-merkledag/test"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	mocknet "github.com/libp2p/go-libp2p/p2p/net/mock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	tmbytes "github.com/tendermint/tendermint/libs/bytes"

	"github.com/celestiaorg/celestia-node/core"
)

func TestServiceLifecycleManagement(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	// setup auxiliary components
	net, err := mocknet.FullMeshConnected(ctx, 2)
	require.NoError(t, err)

	pub, err := pubsub.NewGossipSub(ctx, net.Hosts()[0], pubsub.WithMessageSignaturePolicy(pubsub.StrictNoSign))
	require.NoError(t, err)

	suite := NewTestSuite(t, 3)
	store, err := NewStoreWithHead(datastore.NewMapDatastore(), suite.Head())
	require.NoError(t, err)

	// create sub services
	ex := NewLocalExchange(store)
	syncer := NewSyncer(ex, store, tmbytes.HexBytes{})
	p2pSub := NewP2PSubscriber(pub, syncer.Validate)
	fake := new(fakeLifecycle)
	lifecycles := []Lifecycle{
		ex,
		syncer,
		p2pSub,
		fake,
	}

	serv := NewHeaderService(lifecycles)

	err = serv.Start(ctx)
	require.NoError(t, err)

	// check to ensure fake lifecycle is `started`
	assert.True(t, fake.started)

	err = serv.Stop(ctx)
	require.NoError(t, err)
}

func TestServiceFailedStart(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	// set up mock client in order to stop it before services are started up
	// to trigger a failure
	mockClient := core.MockEmbeddedClient()
	dag := mdutils.Mock()
	coreEx := NewCoreExchange(core.NewBlockFetcher(mockClient), dag)
	// set up syncer
	suite := NewTestSuite(t, 3)
	store, err := NewStoreWithHead(datastore.NewMapDatastore(), suite.Head())
	require.NoError(t, err)
	syncer := NewSyncer(coreEx, store, tmbytes.HexBytes{})
	// set up p2p subscription
	net, err := mocknet.FullMeshConnected(ctx, 2)
	require.NoError(t, err)
	pub, err := pubsub.NewGossipSub(ctx, net.Hosts()[0], pubsub.WithMessageSignaturePolicy(pubsub.StrictNoSign))
	require.NoError(t, err)
	p2pSub := NewP2PSubscriber(pub, syncer.Validate)

	lifecycles := []Lifecycle{
		syncer,
		p2pSub,
		coreEx,
	}

	serv := NewHeaderService(lifecycles)

	// stop mock client in order to trigger a failure on start for CoreExchange
	err = mockClient.Stop()
	require.NoError(t, err)

	// ensure there is a failure when header Service is started
	err = serv.Start(ctx)
	require.Error(t, err)
}

type fakeLifecycle struct {
	started bool
}

func (f *fakeLifecycle) Start(_ context.Context) error {
	f.started = true
	return nil
}

func (f *fakeLifecycle) Stop(_ context.Context) error {
	f.started = false
	return nil
}
