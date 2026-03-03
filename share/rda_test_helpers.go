package share

import (
	"testing"

	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/stretchr/testify/require"
)

// generateTestPeerID creates a random peer ID for testing
func generateTestPeerID(t *testing.T) peer.ID {
	priv, _, err := crypto.GenerateKeyPair(crypto.RSA, 2048)
	require.NoError(t, err)

	testID, err := peer.IDFromPrivateKey(priv)
	require.NoError(t, err)

	return testID
}

// generateTestPeerIDs creates multiple random peer IDs
func generateTestPeerIDs(t *testing.T, count int) []peer.ID {
	ids := make([]peer.ID, count)
	for i := 0; i < count; i++ {
		ids[i] = generateTestPeerID(t)
	}
	return ids
}
