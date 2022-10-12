package ipld

import (
	mrand "math/rand"
	"testing"

	"github.com/ipfs/go-cid"
	"github.com/stretchr/testify/require"
)

func RandNamespacedCID(t *testing.T) cid.Cid {
	raw := make([]byte, nmtHashSize)
	_, err := mrand.Read(raw)
	require.NoError(t, err)
	id, err := CidFromNamespacedSha256(raw)
	require.NoError(t, err)
	return id
}
