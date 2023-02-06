package p2p

import (
	"context"
	"testing"

	"github.com/ipfs/go-datastore"
	"github.com/stretchr/testify/require"

	"github.com/celestiaorg/celestia-node/libs/header/store"
	"github.com/celestiaorg/celestia-node/libs/header/test"
	"github.com/celestiaorg/celestia-node/nodebuilder/p2p"
)

func TestExchangeServer_handleRequestTimeout(t *testing.T) {
	peer := createMocknet(t, 1)
	s, err := store.NewStore[*test.DummyHeader](datastore.NewMapDatastore())
	require.NoError(t, err)
	server, err := NewExchangeServer[*test.DummyHeader](
		peer[0],
		s,
		WithProtocolSuffix[ServerParameters](string(p2p.Private)),
	)
	require.NoError(t, err)
	err = server.Start(context.Background())
	require.NoError(t, err)
	t.Cleanup(func() {
		server.Stop(context.Background()) //nolint:errcheck
	})

	_, err = server.handleRequest(1, 200)
	require.Error(t, err)
}
