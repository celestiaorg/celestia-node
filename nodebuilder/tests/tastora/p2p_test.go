//go:build integration

package tastora

import (
	"context"
	"testing"
	"time"

	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/stretchr/testify/suite"

	rpcclient "github.com/celestiaorg/celestia-node/api/rpc/client"
)

// P2PTestSuite runs a 2-bridge topology where bridge[1] is connected to bridge[0]
type P2PTestSuite struct {
	suite.Suite
	framework *Framework
}

func TestP2PTestSuite(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping p2p integration tests in short mode")
	}
	suite.Run(t, &P2PTestSuite{})
}

func (s *P2PTestSuite) SetupSuite() {
	s.framework = NewFramework(s.T(), WithValidators(1), WithBridgeNodes(2), WithLightNodes(0))
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	s.Require().NoError(s.framework.SetupNetwork(ctx))
	s.framework.NewBridgeNode(ctx)
}

func (s *P2PTestSuite) TearDownSuite() {
	if s.framework != nil {
		s.framework.Cleanup()
	}
}

// TestP2PBridgeToBridge brings up two bridges and asserts they are mutually connected: bridge[1] is
// wired to bridge[0] as a trusted peer, so each must see the other in its peer set and report a
// Connected state in both directions.
func (s *P2PTestSuite) TestP2PBridgeToBridge() {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	bridges := s.framework.GetBridgeNodes()
	s.Require().Len(bridges, 2, "suite must run with two bridge nodes")

	client0 := s.framework.GetNodeRPCClient(ctx, bridges[0])
	client1 := s.framework.GetNodeRPCClient(ctx, bridges[1])

	info0, err := client0.P2P.Info(ctx)
	s.Require().NoError(err, "bridge[0] should provide P2P info")

	info1, err := client1.P2P.Info(ctx)
	s.Require().NoError(err, "bridge[1] should provide P2P info")
	s.Require().NotEqual(info0.ID, info1.ID, "bridges must have distinct peer IDs")

	s.waitConnected(ctx, client1, info0.ID, "bridge[1] -> bridge[0]")
	s.waitConnected(ctx, client0, info1.ID, "bridge[0] -> bridge[1]")

	peers0, err := client0.P2P.Peers(ctx)
	s.Require().NoError(err, "bridge[0] should list peers")
	s.Assert().Contains(peerIDs(peers0), info1.ID, "bridge[0] peer set should include bridge[1]")

	peers1, err := client1.P2P.Peers(ctx)
	s.Require().NoError(err, "bridge[1] should list peers")
	s.Assert().Contains(peerIDs(peers1), info0.ID, "bridge[1] peer set should include bridge[0]")
}

func (s *P2PTestSuite) waitConnected(ctx context.Context, client *rpcclient.Client, id peer.ID, dir string) {
	s.Require().Eventually(func() bool {
		c, err := client.P2P.Connectedness(ctx, id)
		return err == nil && c == network.Connected
	}, 60*time.Second, time.Second, "%s should reach Connected", dir)
}

func peerIDs(peers []peer.ID) map[peer.ID]struct{} {
	set := make(map[peer.ID]struct{}, len(peers))
	for _, p := range peers {
		set[p] = struct{}{}
	}
	return set
}
