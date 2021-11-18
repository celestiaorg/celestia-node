package header

import (
	"context"
	"testing"

	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestHeaderInjection tests to make sure that starting the header Service
// will perform a request for the ExtendedHeader at the given head hash, and
// inject it as the Head of the header chain upon initialization.
func TestHeaderInjection(t *testing.T) {
	host, peer := createMocknet(t)
	ex, store := createMockExchangeAndStore(t, host, peer)

	// get mock host and create new gossipsub on it
	ps, err := pubsub.NewGossipSub(context.Background(), ex.host,
		pubsub.WithMessageSignaturePolicy(pubsub.StrictNoSign))
	require.NoError(t, err)

	// get the header hash from the actual store, and init a
	// new empty store to trigger the Request for headerHash
	expectedHeader := store.headers[len(store.headers)-1]
	emptyStore := &mockStore{
		headers: make(map[int]*ExtendedHeader),
	}

	serv := NewHeaderService(ex, emptyStore, ps, expectedHeader.Hash())

	err = serv.Start(context.Background())
	require.NoError(t, err)
	// check that head is properly injected
	head, err := emptyStore.Head(context.Background())
	require.NoError(t, err)
	assert.Equal(t, expectedHeader.Height, head.Height)
}
