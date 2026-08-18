//go:build integration

package tastora

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
)

// consensusRetainBlocks is how many recent blocks the consensus node keeps; older blocks
// are pruned from its store, so a bridge must source them from the archival peer over p2p.
// celestia-app floors min-retain-blocks at 3000, so this is the smallest usable value;
// the framework pairs it with fast blocks so the chain clears it within the test.
const consensusRetainBlocks = 3000

// HeaderP2PSyncTestSuite runs a pruned consensus node, an archival bridge (A) that retains
// every block, and a fresh non-archival bridge (B). B must backfill blocks the consensus
// node has already pruned, which it can only obtain from A over the p2p header exchange.
type HeaderP2PSyncTestSuite struct {
	suite.Suite
	framework *Framework
}

func TestHeaderP2PSyncTestSuite(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping header p2p sync integration tests in short mode")
	}
	suite.Run(t, &HeaderP2PSyncTestSuite{})
}

func (s *HeaderP2PSyncTestSuite) SetupSuite() {
	s.framework = NewFramework(s.T(),
		WithValidators(1), WithBridgeNodes(2), WithLightNodes(0),
		WithArchivalBridge(), WithPrunedConsensus(consensusRetainBlocks))
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	s.Require().NoError(s.framework.SetupNetwork(ctx))
}

func (s *HeaderP2PSyncTestSuite) TearDownSuite() {
	if s.framework != nil {
		s.framework.Cleanup()
	}
}

// TestBridgeArchivalHeaderSyncViaP2P asserts a bridge backfills headers the consensus node
// has pruned from an archival bridge over p2p. The consensus node keeps only the last few
// blocks (min-retain-blocks), so when B fetches an early height core has no block and
// core.Exchange falls back to the p2p header exchange (archival A) — logged at INFO.
func (s *HeaderP2PSyncTestSuite) TestBridgeArchivalHeaderSyncViaP2P() {
	ctx, cancel := context.WithTimeout(context.Background(), 12*time.Minute)
	defer cancel()

	archival := s.framework.GetArchivalNode()
	clientA := s.framework.GetNodeRPCClient(ctx, archival)

	// Advance past the retain window so early blocks (incl. prunedHeight) are pruned from
	// the consensus node while archival A keeps them.
	_, err := clientA.Header.WaitForHeight(ctx, consensusRetainBlocks+100)
	s.Require().NoError(err, "chain should advance past the consensus retain window")

	// Fresh non-archival bridge B, peered to archival A (window 0 = no override).
	bridgeB := s.framework.StartBridgeNodeWithSmallWindow(ctx, 0)
	clientB := s.framework.GetNodeRPCClient(ctx, bridgeB)

	headA, err := clientA.Header.LocalHead(ctx)
	s.Require().NoError(err, "should get A's head")
	_, err = clientB.Header.WaitForHeight(ctx, headA.Height())
	s.Require().NoError(err, "B should sync to the head, backfilling pruned blocks from A over p2p")

	// An early height was pruned from the consensus node, so B could only get its header
	// from archival A over p2p; it must match A's canonical header.
	const prunedHeight = 5
	hdrB, err := clientB.Header.GetByHeight(ctx, prunedHeight)
	s.Require().NoError(err, "B should hold the pruned-from-core header at %d", prunedHeight)
	hdrA, err := clientA.Header.GetByHeight(ctx, prunedHeight)
	s.Require().NoError(err, "archival A should hold the header at %d", prunedHeight)
	s.Assert().Equal(hdrA.Hash(), hdrB.Hash(), "B's p2p-fetched header should match A's")

	// B's own logs are the direct evidence the header came from p2p rather than core.
	_, logs := s.framework.bridgeContainerExit(ctx, bridgeB)
	s.Require().Contains(logs, "fetched extended header from p2p (core unavailable)",
		"B should have fetched pruned headers from p2p")
	s.T().Log("confirmed: B fetched pruned-from-core headers via the p2p fallback")
}
