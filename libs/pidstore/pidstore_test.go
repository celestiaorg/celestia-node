package pidstore

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"testing"
	"time"

	"github.com/ipfs/go-datastore"
	"github.com/ipfs/go-datastore/sync"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPutLoad(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer t.Cleanup(cancel)

	peerstore := NewPeerIDStore(sync.MutexWrap(datastore.NewMapDatastore()))

	ids, err := generateRandomPeerList(10)
	require.NoError(t, err)

	err = peerstore.Put(ctx, ids)
	require.NoError(t, err)

	retrievedPeerlist, err := peerstore.Load(ctx)
	require.NoError(t, err)

	assert.Equal(t, len(ids), len(retrievedPeerlist))
	assert.Equal(t, ids, retrievedPeerlist)
}

func generateRandomPeerList(length int) ([]peer.ID, error) {
	peerlist := make([]peer.ID, length)
	for i := range peerlist {
		key, err := rsa.GenerateKey(rand.Reader, 2096)
		if err != nil {
			return nil, err
		}

		_, pubkey, err := crypto.KeyPairFromStdKey(key)
		if err != nil {
			return nil, err
		}

		peerID, err := peer.IDFromPublicKey(pubkey)
		if err != nil {
			return nil, err
		}

		peerlist[i] = peerID
	}

	return peerlist, nil
}
