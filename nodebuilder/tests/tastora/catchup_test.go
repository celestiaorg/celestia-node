//go:build integration

package tastora

import (
	"bytes"
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"

	"github.com/celestiaorg/go-square/v4/share"

	rpcclient "github.com/celestiaorg/celestia-node/api/rpc/client"
	nodeblob "github.com/celestiaorg/celestia-node/blob"
	"github.com/celestiaorg/celestia-node/share/availability"
	"github.com/celestiaorg/celestia-node/state"
)

// catchupWindow is the shrunk availability window given to the catching-up bridge
const catchupWindow = 10 * time.Second

// CatchupTestSuite runs a 2-bridge topology where bridge[0] (A) is archival and bridge[1] (B) is a
// non-archival node started with a shrunk availability window.
type CatchupTestSuite struct {
	suite.Suite
	framework *Framework
}

func TestCatchupTestSuite(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping catchup integration tests in short mode")
	}
	suite.Run(t, &CatchupTestSuite{})
}

func (s *CatchupTestSuite) SetupSuite() {
	s.framework = NewFramework(s.T(),
		WithValidators(1), WithBridgeNodes(2), WithLightNodes(0), WithArchivalBridge())
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	s.Require().NoError(s.framework.SetupNetwork(ctx))
}

func (s *CatchupTestSuite) TearDownSuite() {
	if s.framework != nil {
		s.framework.Cleanup()
	}
}

func (s *CatchupTestSuite) TestBridgeCatchupViaShrex() {
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Minute)
	defer cancel()

	archival := s.framework.GetBridgeNodes()[0]
	clientA := s.framework.GetNodeRPCClient(ctx, archival)

	// Bring B up with a small window, let it catch up to the current head, then take it offline.
	bridgeB := s.framework.StartBridgeNodeWithSmallWindow(ctx, catchupWindow)
	clientB := s.framework.GetNodeRPCClient(ctx, bridgeB)
	headB, err := clientB.Header.LocalHead(ctx)
	s.Require().NoError(err, "B should report a local head before going offline")
	s.Require().NoError(bridgeB.Stop(ctx), "should stop B")

	// While B is offline, submit a blob to A at a height above B's last-seen head.
	namespace, err := share.NewV0Namespace(bytes.Repeat([]byte{0x07}, 10))
	s.Require().NoError(err, "should create namespace")
	nodeAddr, err := clientA.State.AccountAddress(ctx)
	s.Require().NoError(err, "should get A account address")
	libBlob, err := share.NewV1Blob(namespace, []byte("catch me over shrex"), nodeAddr.Bytes())
	s.Require().NoError(err, "should build blob")
	nodeBlobs, err := nodeblob.ToNodeBlobs(libBlob)
	s.Require().NoError(err, "should convert blob")

	txConfig := state.NewTxConfig(state.WithGas(300_000), state.WithGasPrice(5000))
	gapHeight, err := clientA.Blob.Submit(ctx, nodeBlobs, txConfig)
	s.Require().NoError(err, "A should submit the gap blob")
	s.Require().Greater(gapHeight, headB.Height(), "gap blob must land above B's pre-offline head")

	// Age the gap block past B's window (block time ~1s) with margin, and advance the chain beyond it.
	time.Sleep(catchupWindow + 5*time.Second)
	_, err = clientA.Header.WaitForHeight(ctx, gapHeight+15)
	s.Require().NoError(err, "chain should advance past the gap height")

	// Restart B: it backfills the missed headers, but holds the gap header without its EDS.
	s.Require().NoError(bridgeB.Start(ctx), "should restart B")
	clientB = s.framework.GetNodeRPCClient(ctx, bridgeB)
	s.waitNodeUp(ctx, clientB)
	_, err = clientB.Header.WaitForHeight(ctx, gapHeight)
	s.Require().NoError(err, "B should backfill the header at the gap height")

	// The gap height is outside B's sampling window: the DASer skips it and the local availability
	// check refuses it — confirming B neither sampled nor stored it locally.
	err = clientB.Share.SharesAvailable(ctx, gapHeight)
	s.Require().ErrorContains(err, availability.ErrOutsideSamplingWindow.Error(),
		"gap height should be outside B's sampling window")

	// A (archival) serves the canonical square.
	edsA, err := clientA.Share.GetEDS(ctx, gapHeight)
	s.Require().NoError(err, "archival A should serve the gap EDS")

	// GetEDS bypasses the sampling-window gate so the request goes to the A node over shrex
	fetchCtx, fetchCancel := context.WithTimeout(ctx, 2*time.Minute)
	defer fetchCancel()
	edsB, err := clientB.Share.GetEDS(fetchCtx, gapHeight)
	s.Require().NoError(err, "B should fetch the out-of-window EDS from archival A over shrex")
	s.Assert().Equal(edsA.Flattened(), edsB.Flattened(), "B's shrex-fetched square should match A's")
}

func (s *CatchupTestSuite) waitNodeUp(ctx context.Context, client *rpcclient.Client) {
	s.Require().Eventually(func() bool {
		_, err := client.Header.LocalHead(ctx)
		return err == nil
	}, 90*time.Second, 2*time.Second, "B RPC should become reachable after restart")
}
