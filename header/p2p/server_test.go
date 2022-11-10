package p2p

import (
	"context"
	"testing"

	"github.com/ipfs/go-datastore"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/celestiaorg/celestia-node/header/store"
)

func TestExchangeServer_handleRequestTimeout(t *testing.T) {
	_, peer := createMocknet(t)
	s, err := store.NewStore(datastore.NewMapDatastore())
	assert.NoError(t, err)
	server := NewExchangeServer(peer, s, "private")
	err = server.Start(context.Background())
	assert.NoError(t, err)
	t.Cleanup(func() {
		server.Stop(context.Background()) //nolint:errcheck
	})

	_, err = server.handleRequest(1, 200)
	require.Error(t, err)
}
