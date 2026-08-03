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
	"github.com/celestiaorg/celestia-node/state"
)

// archivalAvailabilityWindow is the shrunk availability window for the archival node, so
// a height submitted early ages out of the window within the test's runtime. It must stay
// large enough that the node's own head remains "recent" and its syncer doesn't stall.
const archivalAvailabilityWindow = 2 * time.Minute

// ArchivalTestSuite runs a bridge node started with `--archival` and a shrunk availability
// window, so it must retain and serve block data even after it falls outside that window.
type ArchivalTestSuite struct {
	suite.Suite
	framework *Framework
}

func TestArchivalTestSuite(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping archival integration tests in short mode")
	}
	suite.Run(t, &ArchivalTestSuite{})
}

func (s *ArchivalTestSuite) SetupSuite() {
	s.framework = NewFramework(s.T(), WithValidators(1), WithBridgeNodes(1), WithLightNodes(0),
		WithArchivalBridge(), WithAvailabilityWindow(archivalAvailabilityWindow))
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	s.Require().NoError(s.framework.SetupNetwork(ctx))
}

func (s *ArchivalTestSuite) TearDownSuite() {
	if s.framework != nil {
		s.framework.Cleanup()
	}
}

// TestArchivalNodeKeepsData submits a blob at an early height, lets the chain advance
// until that height is outside the (shrunk) availability window, and asserts the archival
// node still serves the header, shares and blob. On an archival node SharesAvailable
// bypasses the window gate, whereas a non-archival node would reject an out-of-window
// height and prune its data — so this exercises the archival retention guarantee.
func (s *ArchivalTestSuite) TestArchivalNodeKeepsData() {
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Minute)
	defer cancel()

	bridge := s.framework.GetBridgeNodes()[0]
	client := s.framework.GetNodeRPCClient(ctx, bridge)

	namespace, err := share.NewV0Namespace(bytes.Repeat([]byte{0x05}, 10))
	s.Require().NoError(err, "should create namespace")

	blobData := []byte("TestArchivalNodeKeepsData: archival retention test data")
	nodeBlobs := s.createBlob(ctx, client, namespace, blobData)

	txConfig := state.NewTxConfig(state.WithGas(300_000), state.WithGasPrice(5000))
	earlyHeight, err := client.Blob.Submit(ctx, nodeBlobs, txConfig)
	s.Require().NoError(err, "should submit blob")
	s.Require().NotZero(earlyHeight)

	_, err = client.Header.WaitForHeight(ctx, earlyHeight)
	s.Require().NoError(err, "should reach submit height")

	earlyHdr, err := client.Header.GetByHeight(ctx, earlyHeight)
	s.Require().NoError(err, "should get early header")

	// Advance well past the availability window (block time ~1s, window 2m) so the early
	// height is firmly outside it.
	_, err = client.Header.WaitForHeight(ctx, earlyHeight+140)
	s.Require().NoError(err, "chain should advance past the availability window")

	postHdr, err := client.Header.GetByHeight(ctx, earlyHeight)
	s.Require().NoError(err, "early header should still be served after advancing")
	s.Assert().Equal(earlyHdr.Hash(), postHdr.Hash(), "early header should be unchanged")
	s.Assert().Equal(earlyHdr.DAH.Hash(), postHdr.DAH.Hash(), "early DAH should be unchanged")

	s.Require().NoError(client.Share.SharesAvailable(ctx, earlyHeight),
		"early-height shares should stay available on an archival node")

	eds, err := client.Share.GetEDS(ctx, earlyHeight)
	s.Require().NoError(err, "early-height EDS should stay retrievable")
	s.Require().NotNil(eds)

	got, err := client.Blob.Get(ctx, earlyHeight, namespace, nodeBlobs[0].Commitment)
	s.Require().NoError(err, "early-height blob should stay retrievable")
	s.Require().NotNil(got)
	s.Assert().Equal(blobData, bytes.TrimRight(got.Data(), "\x00"), "blob should round-trip")
}

func (s *ArchivalTestSuite) createBlob(
	ctx context.Context,
	client *rpcclient.Client,
	namespace share.Namespace,
	data []byte,
) []*nodeblob.Blob {
	nodeAddr, err := client.State.AccountAddress(ctx)
	s.Require().NoError(err, "should get node address")

	libBlob, err := share.NewV1Blob(namespace, data, nodeAddr.Bytes())
	s.Require().NoError(err, "should create libshare blob")

	nodeBlobs, err := nodeblob.ToNodeBlobs(libBlob)
	s.Require().NoError(err, "should convert to node blobs")
	return nodeBlobs
}
