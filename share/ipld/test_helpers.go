package ipld

import (
	"crypto/rand"
	"testing"

	"github.com/ipfs/go-cid"
	"github.com/stretchr/testify/require"
)

func RandNamespacedCID(t *testing.T) cid.Cid {
	raw := make([]byte, NmtHashSize)
	_, err := rand.Read(raw)
	require.NoError(t, err)
	id, err := CidFromNamespacedSha256(raw)
	require.NoError(t, err)
	return id
}
