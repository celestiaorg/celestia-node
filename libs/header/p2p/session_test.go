package p2p

import (
	"context"
	"testing"
	"time"

	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/celestiaorg/celestia-node/libs/header/test"
)

func Test_PrepareRequests(t *testing.T) {
	// from : 1, amount: 10, headersPerPeer: 5
	// result -> {{1, 2, 3, 4, 5}, {6, 7, 8, 9, 10}}
	requests := prepareRequests(1, 10, 5)
	require.Len(t, requests, 2)
	require.Equal(t, requests[0].GetOrigin(), uint64(1))
	require.Equal(t, requests[1].GetOrigin(), uint64(6))
}

// Test_Validate ensures that headers range is adjacent and valid.
func Test_Validate(t *testing.T) {
	suite := test.NewTestSuite(t)
	head := suite.Head()
	ses := newSession(
		context.Background(),
		nil,
		&peerTracker{trackedPeers: make(map[peer.ID]*peerStat)},
		"", time.Second,
		withValidation(head),
	)

	headers := suite.GenDummyHeaders(5)
	err := ses.validate(headers)
	assert.NoError(t, err)
}

// Test_ValidateFails ensures that non-adjacent range will return an error.
func Test_ValidateFails(t *testing.T) {
	suite := test.NewTestSuite(t)
	head := suite.Head()
	ses := newSession(
		context.Background(),
		nil,
		&peerTracker{trackedPeers: make(map[peer.ID]*peerStat)},
		"", time.Second,
		withValidation(head),
	)

	headers := suite.GenDummyHeaders(5)
	// break adjacency
	headers[2] = headers[4]
	err := ses.validate(headers)
	assert.Error(t, err)
}
