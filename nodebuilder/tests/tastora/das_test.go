//go:build integration

package tastora

import (
	"bytes"
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"

	"github.com/celestiaorg/go-square/v4/share"

	nodeblob "github.com/celestiaorg/celestia-node/blob"
	"github.com/celestiaorg/celestia-node/state"
)

// DASTestSuite runs verifies that sampling works correctly.
type DASTestSuite struct {
	suite.Suite
	framework *Framework
}

func TestDASTestSuite(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping DAS integration tests in short mode")
	}
	suite.Run(t, &DASTestSuite{})
}

func (s *DASTestSuite) SetupSuite() {
	s.framework = NewFramework(s.T(), WithValidators(1), WithBridgeNodes(1), WithLightNodes(1))
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	s.Require().NoError(s.framework.SetupNetwork(ctx))
	s.framework.NewLightNodeBitswapOff(ctx)
}

func (s *DASTestSuite) TearDownSuite() {
	if s.framework != nil {
		s.framework.Cleanup()
	}
}

// TestBasicDASFlow asserts the data-availability sampling pipeline over shrex
func (s *DASTestSuite) TestBasicDASFlow() {
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Minute)
	defer cancel()

	bridge := s.framework.GetBridgeNodes()[0]
	light := s.framework.GetLightNodes()[0]
	bridgeClient := s.framework.GetNodeRPCClient(ctx, bridge)
	lightClient := s.framework.GetNodeRPCClient(ctx, light)

	namespace, err := share.NewV0Namespace(bytes.Repeat([]byte{0x09}, 10))
	s.Require().NoError(err, "should create namespace")

	nodeAddr, err := bridgeClient.State.AccountAddress(ctx)
	s.Require().NoError(err, "should get bridge account address")

	libBlob, err := share.NewV1Blob(namespace, []byte("sample me over shrex"), nodeAddr.Bytes())
	s.Require().NoError(err, "should build blob")

	nodeBlobs, err := nodeblob.ToNodeBlobs(libBlob)
	s.Require().NoError(err, "should convert blob")

	txConfig := state.NewTxConfig(state.WithGas(300_000), state.WithGasPrice(5000))
	blobHeight, err := bridgeClient.Blob.Submit(ctx, nodeBlobs, txConfig)
	s.Require().NoError(err, "bridge should submit blob")
	s.Require().NotZero(blobHeight, "blob submission should return a valid height")

	_, err = bridgeClient.Header.WaitForHeight(ctx, blobHeight+5)
	s.Require().NoError(err, "chain should advance past the blob height")

	// wait until it has the blob height, then sample.
	_, err = lightClient.Header.WaitForHeight(ctx, blobHeight)
	s.Require().NoError(err, "light node should sync the header at the blob height")

	catchupCtx, catchupCancel := context.WithTimeout(ctx, 3*time.Minute)
	defer catchupCancel()
	s.Require().NoError(lightClient.DAS.WaitCatchUp(catchupCtx), "light node DASer should catch up")

	stats, err := lightClient.DAS.SamplingStats(ctx)
	s.Require().NoError(err, "should get light node sampling stats")
	s.T().Logf("light DAS stats: sampledChainHead=%d catchupHead=%d networkHead=%d failed=%d isRunning=%v catchUpDone=%v",
		stats.SampledChainHead, stats.CatchupHead, stats.NetworkHead, len(stats.Failed), stats.IsRunning, stats.CatchUpDone)
	s.Assert().True(stats.IsRunning, "DASer should be running")
	s.Assert().True(stats.CatchUpDone, "DASer should have finished catching up")
	s.Assert().Empty(stats.Failed, "no sampled height should have failed")
	s.Assert().GreaterOrEqual(stats.SampledChainHead, blobHeight,
		"sampled chain head should reach at least the blob height %d", blobHeight)

	// sample the non-empty blob square over shrex.
	s.Require().NoError(lightClient.Share.SharesAvailable(ctx, blobHeight),
		"blob square shares should be available to the light node at height %d", blobHeight)

	// the light node does not store squares, so GetEDS forces a shrex to get it
	// from the bridge.
	edsBridge, err := bridgeClient.Share.GetEDS(ctx, blobHeight)
	s.Require().NoError(err, "bridge should serve the blob EDS")
	fetchCtx, fetchCancel := context.WithTimeout(ctx, 2*time.Minute)
	defer fetchCancel()
	edsLight, err := lightClient.Share.GetEDS(fetchCtx, blobHeight)
	s.Require().NoError(err, "light node should reconstruct the EDS from the bridge over shrex")
	s.Assert().Equal(edsBridge.Flattened(), edsLight.Flattened(),
		"light node's shrex-fetched square should match the bridge's")
}
