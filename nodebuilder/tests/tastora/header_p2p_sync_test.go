//go:build integration

package tastora

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
)

// archivalHeaderWindow is the shrunk availability window given to the catching-up
// bridge.
const archivalHeaderWindow = 2 * time.Minute

// historyHeight is how much chain the archival bridge builds before the small-window
// bridge joins.
const historyHeight = 150

// HeaderP2PSyncTestSuite runs a 2-bridge topology where bridge[0] (A) is archival and
// bridge[1] (B) is non-archival with a small availability window.
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
		WithValidators(1), WithBridgeNodes(2), WithLightNodes(0), WithArchivalBridge())
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	s.Require().NoError(s.framework.SetupNetwork(ctx))
}

func (s *HeaderP2PSyncTestSuite) TearDownSuite() {
	if s.framework != nil {
		s.framework.Cleanup()
	}
}

// TestBridgeArchivalHeaderSyncViaP2P asserts a non-archival bridge fetches headers that
// are older than its availability window from an archival bridge over p2p.
func (s *HeaderP2PSyncTestSuite) TestBridgeArchivalHeaderSyncViaP2P() {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	archival := s.framework.GetBridgeNodes()[0]
	clientA := s.framework.GetNodeRPCClient(ctx, archival)

	_, err := clientA.Header.WaitForHeight(ctx, historyHeight)
	s.Require().NoError(err, "chain should build enough history before B joins")

	// the early out-of-window heights route to the p2p header exchange and
	// the recent ones to core.
	bridgeB := s.framework.StartBridgeNodeWithSmallWindow(ctx, archivalHeaderWindow)
	clientB := s.framework.GetNodeRPCClient(ctx, bridgeB)

	headA, err := clientA.Header.LocalHead(ctx)
	s.Require().NoError(err, "should get A's head")
	_, err = clientB.Header.WaitForHeight(ctx, headA.Height())
	s.Require().NoError(err, "B should sync to the head")

	_, logs := s.framework.bridgeContainerExit(ctx, bridgeB)
	s.Require().Contains(logs, "range from p2p network",
		"B's logs should show it fetched out-of-window headers over p2p")
	s.T().Log("confirmed: B fetched archival headers via the p2p route")
}
