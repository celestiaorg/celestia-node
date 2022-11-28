package p2p

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func Test_PrepareRequests(t *testing.T) {
	// from : 1, amount: 10, headersPerPeer: 5
	// result -> {{1, 2, 3, 4, 5}, {6, 7, 8, 9, 10}}
	requests := prepareRequests(1, 10, 5)
	require.Len(t, requests, 2)
	require.Equal(t, requests[0].GetOrigin(), uint64(1))
	require.Equal(t, requests[1].GetOrigin(), uint64(6))
}
