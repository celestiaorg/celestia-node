//go:build integration

package tastora

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"

	rpcclient "github.com/celestiaorg/celestia-node/api/rpc/client"
)

// RestartTestSuite verifies a bridge node survives a container restart on a single-bridge
// topology
type RestartTestSuite struct {
	suite.Suite
	framework *Framework
}

func TestRestartTestSuite(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping restart integration tests in short mode")
	}
	suite.Run(t, &RestartTestSuite{})
}

func (s *RestartTestSuite) SetupSuite() {
	s.framework = NewFramework(s.T(), WithValidators(1), WithBridgeNodes(1), WithLightNodes(0))
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	s.Require().NoError(s.framework.SetupNetwork(ctx))
}

func (s *RestartTestSuite) TearDownSuite() {
	if s.framework != nil {
		s.framework.Cleanup()
	}
}

// TestRestartPersistence asserts a bridge node keeps its state across a container restart.
func (s *RestartTestSuite) TestRestartPersistence() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	bridge := s.framework.GetBridgeNodes()[0]
	client := s.framework.GetNodeRPCClient(ctx, bridge)

	head, err := client.Header.LocalHead(ctx)
	s.Require().NoError(err, "should get local head before restart")
	h0 := head.Height()

	preHdr, err := client.Header.GetByHeight(ctx, h0)
	s.Require().NoError(err, "should get header at height %d before restart", h0)

	_, dasErr := client.DAS.SamplingStats(ctx)
	s.Require().NoError(dasErr, "DAS not available", dasErr)

	s.Require().NoError(client.DAS.WaitCatchUp(ctx), "DASer should catch up before restart")
	preStats, err := client.DAS.SamplingStats(ctx)
	s.Require().NoError(err, "should get sampling stats before restart")
	s.T().Logf("pre-restart: head=%d sampledChainHead=%d catchupHead=%d networkHead=%d",
		h0, preStats.SampledChainHead, preStats.CatchupHead, preStats.NetworkHead)

	s.framework.RestartBridgeNode(ctx, bridge)

	client = s.framework.GetNodeRPCClient(ctx, bridge)
	s.waitNodeUp(ctx, client)

	// verify that store is survived after restsart
	postHdr, err := client.Header.GetByHeight(ctx, h0)
	s.Require().NoError(err, "should get header at height %d after restart", h0)
	s.Assert().Equal(preHdr.Hash(), postHdr.Hash(), "header hash at height %d should survive restart", h0)
	s.Assert().Equal(preHdr.DAH.Hash(), postHdr.DAH.Hash(), "DAH at height %d should survive restart", h0)
	s.Require().NoError(client.Share.SharesAvailable(ctx, h0),
		"shares at height %d should stay available after restart", h0)

	immStats, err := client.DAS.SamplingStats(ctx)
	s.Require().NoError(err, "should get sampling stats after restart")
	s.T().Logf("post-restart (immediate): sampledChainHead=%d catchupHead=%d networkHead=%d isRunning=%v",
		immStats.SampledChainHead, immStats.CatchupHead, immStats.NetworkHead, immStats.IsRunning)

	s.Require().NoError(client.DAS.WaitCatchUp(ctx), "DASer should catch up again after restart")
	postStats, err := client.DAS.SamplingStats(ctx)
	s.Require().NoError(err, "should get sampling stats after catch up")
	s.T().Logf("post-restart (caught up): sampledChainHead=%d catchupHead=%d networkHead=%d",
		postStats.SampledChainHead, postStats.CatchupHead, postStats.NetworkHead)
	s.Assert().GreaterOrEqual(postStats.SampledChainHead, preStats.SampledChainHead,
		"sampled chain head should not regress across restart")

	// the node keeps advancing after the restart.
	_, err = client.Header.WaitForHeight(ctx, h0+3)
	s.Require().NoError(err, "node should keep advancing after restart")
}

func (s *RestartTestSuite) waitNodeUp(ctx context.Context, client *rpcclient.Client) {
	s.Require().Eventually(func() bool {
		_, err := client.Header.LocalHead(ctx)
		return err == nil
	}, 90*time.Second, 2*time.Second, "node RPC should become reachable after restart")
}
