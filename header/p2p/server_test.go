package p2p

import (
	"context"
	"testing"

	"github.com/ipfs/go-datastore"
	"github.com/stretchr/testify/require"

	"github.com/celestiaorg/celestia-node/header/store"
)

func TestExchangeServer_handleRequestTimeout(t *testing.T) {
	_, peer := createMocknet(t)
	s, err := store.NewStore(datastore.NewMapDatastore())
	require.NoError(t, err)
	server := NewExchangeServer(peer, s, "private")
	err = server.Start(context.Background())
	require.NoError(t, err)
	t.Cleanup(func() {
		server.Stop(context.Background()) //nolint:errcheck
	})

	_, err = server.handleRequest(1, 200)
	require.Error(t, err)
}
