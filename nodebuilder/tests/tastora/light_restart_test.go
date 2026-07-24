//go:build integration

package tastora

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"

	rpcclient "github.com/celestiaorg/celestia-node/api/rpc/client"
)

// LightRestartTestSuite verifies a light node resumes data-availability sampling.
type LightRestartTestSuite struct {
	suite.Suite
	framework *Framework
}

func TestLightRestartTestSuite(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping light-restart integration tests in short mode")
	}
	suite.Run(t, &LightRestartTestSuite{})
}

func (s *LightRestartTestSuite) SetupSuite() {
	s.framework = NewFramework(s.T(), WithValidators(1), WithBridgeNodes(1), WithLightNodes(1))
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	s.Require().NoError(s.framework.SetupNetwork(ctx))
	s.framework.NewLightNode(ctx)
}

func (s *LightRestartTestSuite) TearDownSuite() {
	if s.framework != nil {
		s.framework.Cleanup()
	}
}

// TestLightNodeResumesSamplingAfterRestart samples the chain on a light node, restarts its container,
// and asserts the light node comes back, keeps its header store, resumes sampling from where it left
// off and keeps advancing.
func (s *LightRestartTestSuite) TestLightNodeResumesSamplingAfterRestart() {
	ctx, cancel := context.WithTimeout(context.Background(), 6*time.Minute)
	defer cancel()

	light := s.framework.GetLightNodes()[0]
	client := s.framework.GetNodeRPCClient(ctx, light)

	head, err := client.Header.LocalHead(ctx)
	s.Require().NoError(err, "should get light local head before restart")
	h0 := head.Height()
	preHdr, err := client.Header.GetByHeight(ctx, h0)
	s.Require().NoError(err, "should get header at height %d before restart", h0)

	s.Require().NoError(client.DAS.WaitCatchUp(ctx), "light DASer should catch up before restart")
	preStats, err := client.DAS.SamplingStats(ctx)
	s.Require().NoError(err, "should get sampling stats before restart")
	s.Require().Empty(preStats.Failed, "no height should have failed before restart")
	s.T().Logf("pre-restart: head=%d sampledChainHead=%d networkHead=%d",
		h0, preStats.SampledChainHead, preStats.NetworkHead)

	s.framework.RestartNode(ctx, light)

	client = s.framework.GetNodeRPCClient(ctx, light)
	s.waitNodeUp(ctx, client)

	postHdr, err := client.Header.GetByHeight(ctx, h0)
	s.Require().NoError(err, "should get header at height %d after restart", h0)
	s.Assert().Equal(preHdr.Hash(), postHdr.Hash(), "header hash at height %d should survive restart", h0)

	immStats, err := client.DAS.SamplingStats(ctx)
	s.Require().NoError(err, "should get sampling stats after restart")
	s.T().Logf("post-restart (immediate): sampledChainHead=%d networkHead=%d isRunning=%v",
		immStats.SampledChainHead, immStats.NetworkHead, immStats.IsRunning)

	s.Require().NoError(client.DAS.WaitCatchUp(ctx), "light DASer should catch up again after restart")
	postStats, err := client.DAS.SamplingStats(ctx)
	s.Require().NoError(err, "should get sampling stats after catch up")
	s.T().Logf("post-restart (caught up): sampledChainHead=%d networkHead=%d",
		postStats.SampledChainHead, postStats.NetworkHead)
	s.Assert().Empty(postStats.Failed, "no height should have failed after restart")
	s.Assert().GreaterOrEqual(postStats.SampledChainHead, preStats.SampledChainHead,
		"sampled chain head should not regress across restart")

	// the light node keeps sampling new blocks after the restart.
	newHead := postStats.NetworkHead + 3
	_, err = client.Header.WaitForHeight(ctx, newHead)
	s.Require().NoError(err, "light node should keep syncing headers after restart")
	s.Require().NoError(client.DAS.WaitCatchUp(ctx), "light DASer should sample past the restart")
	finalStats, err := client.DAS.SamplingStats(ctx)
	s.Require().NoError(err, "should get final sampling stats")
	s.Assert().GreaterOrEqual(finalStats.SampledChainHead, newHead,
		"light node should sample blocks produced after the restart")
	s.Assert().Empty(finalStats.Failed, "no height should have failed while sampling past the restart")
}

func (s *LightRestartTestSuite) waitNodeUp(ctx context.Context, client *rpcclient.Client) {
	s.Require().Eventually(func() bool {
		_, err := client.Header.LocalHead(ctx)
		return err == nil
	}, 90*time.Second, 2*time.Second, "light node RPC should become reachable after restart")
}
