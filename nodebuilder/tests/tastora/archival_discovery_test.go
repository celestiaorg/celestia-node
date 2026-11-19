//go:build integration

package tastora

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"

	"github.com/libp2p/go-libp2p/core/network"
	"github.com/stretchr/testify/suite"

	"github.com/celestiaorg/go-square/v4/share"

	nodeblob "github.com/celestiaorg/celestia-node/blob"
	"github.com/celestiaorg/celestia-node/share/availability"
	"github.com/celestiaorg/celestia-node/state"
)

// discoveryWindow is the availability window of the pruned bridge and the light node.
const discoveryWindow = 30 * time.Second

// ArchivalDiscoveryTestSuite runs a pruned bridge[0], an archival bridge[1] peered only to
// bridge[0], and a light node bootstrapped only from bridge[0].
type ArchivalDiscoveryTestSuite struct {
	suite.Suite
	framework *Framework
}

func TestArchivalDiscoveryTestSuite(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping archival discovery integration tests in short mode")
	}
	suite.Run(t, &ArchivalDiscoveryTestSuite{})
}

func (s *ArchivalDiscoveryTestSuite) SetupSuite() {
	s.framework = NewFramework(s.T(), WithValidators(1), WithBridgeNodes(2), WithLightNodes(1),
		WithAvailabilityWindow(discoveryWindow))
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	s.Require().NoError(s.framework.SetupNetwork(ctx))
	s.framework.StartArchivalBridgeNode(ctx)
	s.framework.NewLightNode(ctx)
}

func (s *ArchivalDiscoveryTestSuite) TearDownSuite() {
	if s.framework != nil {
		s.framework.Cleanup()
	}
}

// TestLightFetchesHistoricalBlobFromArchival asserts the light node discovers the archival bridge
// on its own and retrieves a blob from it once the height falls outside the light's window, where
// requests are routed only to peers found under the archival discovery tag.
func (s *ArchivalDiscoveryTestSuite) TestLightFetchesHistoricalBlobFromArchival() {
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Minute)
	defer cancel()

	prunedClient := s.framework.GetNodeRPCClient(ctx, s.framework.GetBridgeNodes()[0])
	archivalClient := s.framework.GetNodeRPCClient(ctx, s.framework.GetBridgeNodes()[1])
	lightClient := s.framework.GetNodeRPCClient(ctx, s.framework.GetLightNodes()[0])

	namespace, err := share.NewV0Namespace(bytes.Repeat([]byte{0x0D}, 10))
	s.Require().NoError(err)
	nodeAddr, err := prunedClient.State.AccountAddress(ctx)
	s.Require().NoError(err)
	data := []byte("historical blob served by an archival peer")
	libBlob, err := share.NewV1Blob(namespace, data, nodeAddr.Bytes())
	s.Require().NoError(err)
	nodeBlobs, err := nodeblob.ToNodeBlobs(libBlob)
	s.Require().NoError(err)

	txConfig := state.NewTxConfig(state.WithGas(300_000), state.WithGasPrice(5000))
	height, err := prunedClient.Blob.Submit(ctx, nodeBlobs, txConfig)
	s.Require().NoError(err, "pruned bridge should submit blob")
	_, err = archivalClient.Header.WaitForHeight(ctx, height)
	s.Require().NoError(err, "archival bridge should sync the blob height")
	_, err = lightClient.Header.WaitForHeight(ctx, height)
	s.Require().NoError(err, "light should sync the blob height")

	archivalInfo, err := archivalClient.P2P.Info(ctx)
	s.Require().NoError(err)
	s.Require().Eventually(func() bool {
		c, err := lightClient.P2P.Connectedness(ctx, archivalInfo.ID)
		return err == nil && c == network.Connected
	}, 2*time.Minute, time.Second, "light should discover the archival bridge it was not bootstrapped with")

	s.Require().Eventually(func() bool {
		err := lightClient.Share.SharesAvailable(ctx, height)
		return err != nil && strings.Contains(err.Error(), availability.ErrOutsideSamplingWindow.Error())
	}, discoveryWindow+time.Minute, 2*time.Second, "blob height should fall outside the light's window")

	got, err := lightClient.Blob.Get(ctx, height, namespace, nodeBlobs[0].Commitment)
	s.Require().NoError(err, "light should fetch the historical blob from the archival bridge")
	s.Assert().Equal(data, bytes.TrimRight(got.Data(), "\x00"))

	edsArchival, err := archivalClient.Share.GetEDS(ctx, height)
	s.Require().NoError(err)
	edsLight, err := lightClient.Share.GetEDS(ctx, height)
	s.Require().NoError(err, "light should fetch the historical EDS from the archival bridge")
	s.Assert().Equal(edsArchival.Flattened(), edsLight.Flattened())
}
