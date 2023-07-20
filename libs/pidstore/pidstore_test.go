package pidstore

import (
	"context"
	"testing"
	"time"

	"github.com/ipfs/go-datastore"
	"github.com/ipfs/go-datastore/sync"
	libhost "github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	mocknet "github.com/libp2p/go-libp2p/p2p/net/mock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPutLoad(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer t.Cleanup(cancel)

	mn, err := mocknet.FullMeshConnected(5)
	require.NoError(t, err)

	addrinfos := make([]peer.AddrInfo, 5)
	for i, host := range mn.Hosts() {
		addrinfos[i] = *libhost.InfoFromHost(host)
	}

	peerstore := NewPeerIDStore(sync.MutexWrap(datastore.NewMapDatastore()))

	err = peerstore.Put(ctx, addrinfos)
	require.NoError(t, err)

	retrievedPeerlist, err := peerstore.Load(ctx)
	require.NoError(t, err)

	assert.Equal(t, len(addrinfos), len(retrievedPeerlist))
	assert.Equal(t, addrinfos, retrievedPeerlist)
}
