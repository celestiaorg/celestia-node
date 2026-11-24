//go:build integration

package tastora

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"

	"github.com/celestiaorg/go-square/v4/share"

	rpcclient "github.com/celestiaorg/celestia-node/api/rpc/client"
	nodeblob "github.com/celestiaorg/celestia-node/blob"
	nodeshare "github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/celestia-node/share/shwap"
	"github.com/celestiaorg/celestia-node/state"
)

// archivalAvailabilityWindow shrinks the window so an early height ages out during the test,
// but stays large enough that the node's head stays recent and the syncer doesn't stall.
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

// TestArchivalNodeKeepsData submits a blob, advances past the shrunk availability window,
// and asserts the archival node still serves the header, shares and blob — a non-archival
// node would reject the out-of-window height and prune its data.
func (s *ArchivalTestSuite) TestArchivalNodeKeepsData() {
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Minute)
	defer cancel()

	bridge := s.framework.GetArchivalNode()
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

	s.assertQ4Trimmed(ctx, client, earlyHeight, earlyHdr.DAH)
}

// assertQ4Trimmed checks that once a height ages out of the window the archival node keeps only
// the ODS: the first half of the square is still served, while rows and shares that would need
// reconstruction from the second half are reported as not found instead of being computed.
func (s *ArchivalTestSuite) assertQ4Trimmed(
	ctx context.Context,
	client *rpcclient.Client,
	height uint64,
	dah *nodeshare.AxisRoots,
) {
	last := len(dah.RowRoots) - 1

	row, err := client.Share.GetRow(ctx, height, 0)
	s.Require().NoError(err, "first half row should be served from the ODS")
	s.Require().NoError(row.Verify(dah, 0))

	_, err = client.Share.GetShare(ctx, height, last, 0)
	s.Require().NoError(err, "third quadrant share is derived from its column in the ODS")

	// Q4 is trimmed by the pruner, so allow it some time after the height leaves the window.
	s.Require().Eventually(func() bool {
		_, err := client.Share.GetRow(ctx, height, last)
		return err != nil && strings.Contains(err.Error(), shwap.ErrNotFound.Error())
	}, 2*time.Minute, 2*time.Second, "second half row should be reported as not found")

	_, err = client.Share.GetShare(ctx, height, last, last)
	s.Require().ErrorContains(err, shwap.ErrNotFound.Error(), "fourth quadrant share should not be reconstructed")
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
