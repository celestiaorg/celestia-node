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

// E2ESanityTestSuite provides E2E sanity testing of basic bridge-node flows that don't
// warrant a dedicated suite: blob subscription, large blocks, and out-of-range errors.
type E2ESanityTestSuite struct {
	suite.Suite
	framework *Framework
	timeout   time.Duration
}

func TestE2ESanityTestSuite(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping E2E sanity integration tests in short mode")
	}
	suite.Run(t, &E2ESanityTestSuite{timeout: 2 * time.Minute})
}

// withTimeout creates a context with the suite's configured timeout
func (s *E2ESanityTestSuite) withTimeout() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), s.timeout)
}

func (s *E2ESanityTestSuite) SetupSuite() {
	s.framework = NewFramework(s.T(), WithValidators(1), WithBridgeNodes(1), WithLightNodes(0))
	ctx, cancel := s.withTimeout()
	defer cancel()
	s.Require().NoError(s.framework.SetupNetwork(ctx))
}

func (s *E2ESanityTestSuite) TearDownSuite() {
	if s.framework != nil {
		s.framework.Cleanup()
	}
}

// TestBlobSubscribe subscribes to a namespace, submits a blob to it and wait
// until blob appears in the chain
func (s *E2ESanityTestSuite) TestBlobSubscribe() {
	ctx, cancel := s.withTimeout()
	defer cancel()

	bridgeNode := s.framework.GetBridgeNodes()[0]
	bridgeClient := s.framework.GetNodeRPCClient(ctx, bridgeNode)
	wsClient := s.framework.GetNodeRPCClientWS(ctx, bridgeNode)
	defer wsClient.Close()

	namespace, err := share.NewV0Namespace(bytes.Repeat([]byte{0x03}, 10))
	s.Require().NoError(err, "should create namespace")

	sub, err := wsClient.Blob.Subscribe(ctx, namespace)
	s.Require().NoError(err, "should subscribe to blobs")

	blobData := []byte("TestBlobSubscribe: blob subscription test data")
	nodeBlobs := s.createBlobsForSubmission(ctx, bridgeClient, namespace, blobData)

	txConfig := state.NewTxConfig(state.WithGas(300_000), state.WithGasPrice(5000))
	height, err := bridgeClient.Blob.Submit(ctx, nodeBlobs, txConfig)
	s.Require().NoError(err, "bridge node should be able to submit blob")
	s.Require().NotZero(height, "blob submission should return valid height")

	for {
		select {
		case resp := <-sub:
			s.Require().NotNil(resp)
			for _, b := range resp.Blobs {
				if bytes.Equal(b.Commitment, nodeBlobs[0].Commitment) {
					return
				}
			}
		case <-ctx.Done():
			s.FailNowf("timed out waiting for blob subscription event", "blob at height %d never delivered", height)
		}
	}
}

// TestLargeBlock submits a large blob that spans multiple rows.
func (s *E2ESanityTestSuite) TestLargeBlock() {
	ctx, cancel := s.withTimeout()
	defer cancel()

	bridgeNode := s.framework.GetBridgeNodes()[0]
	bridgeClient := s.framework.GetNodeRPCClient(ctx, bridgeNode)

	namespace, err := share.NewV0Namespace(bytes.Repeat([]byte{0x04}, 10))
	s.Require().NoError(err, "should create namespace")

	blobData := bytes.Repeat([]byte("celestia-large-block"), 12_800) // ~256 KiB
	nodeBlobs := s.createBlobsForSubmission(ctx, bridgeClient, namespace, blobData)

	txConfig := state.NewTxConfig(state.WithGasPrice(0.1))
	height, err := bridgeClient.Blob.Submit(ctx, nodeBlobs, txConfig)
	s.Require().NoError(err, "bridge node should be able to submit large blob")
	s.Require().NotZero(height, "blob submission should return valid height")

	_, err = bridgeClient.Header.WaitForHeight(ctx, height)
	s.Require().NoError(err, "bridge node should be able to wait for block inclusion")

	err = bridgeClient.Share.SharesAvailable(ctx, height)
	s.Require().NoError(err, "large-block shares should be available at height %d", height)

	hdr, err := bridgeClient.Header.GetByHeight(ctx, height)
	s.Require().NoError(err, "should get header at height %d", height)
	s.Assert().Greater(len(hdr.DAH.RowRoots), share.MinSquareSize*2,
		"large blob should grow the EDS past the minimum empty-square width")

	retrievedBlob, err := bridgeClient.Blob.Get(ctx, height, namespace, nodeBlobs[0].Commitment)
	s.Require().NoError(err, "bridge node should be able to retrieve large blob")
	s.Require().NotNil(retrievedBlob, "bridge node should return valid blob")

	retrievedData := bytes.TrimRight(retrievedBlob.Data(), "\x00")
	s.Assert().Equal(blobData, retrievedData, "retrieved large blob should round-trip the payload")
}

// TestErrorPaths asserts the RPC surface returns a clean and bounded error for out-of-range height
func (s *E2ESanityTestSuite) TestErrorPaths() {
	ctx, cancel := s.withTimeout()
	defer cancel()

	bridgeNode := s.framework.GetBridgeNodes()[0]
	bridgeClient := s.framework.GetNodeRPCClient(ctx, bridgeNode)

	head, err := bridgeClient.Header.LocalHead(ctx)
	s.Require().NoError(err, "should get local head")

	cases := []struct {
		name   string
		height uint64
	}{
		{"FutureHeight", head.Height() + 1_000_000},
		{"BelowGenesis", 0},
	}

	for _, tc := range cases {
		s.Run(tc.name, func() {
			// set timeout for each probe far under the suite timeout
			probe := func(name string, call func(context.Context) error) {
				probeCtx, probeCancel := context.WithTimeout(ctx, 15*time.Second)
				defer probeCancel()

				callErr := call(probeCtx)
				s.Require().Error(callErr, "%s(%d) should return an error", name, tc.height)
				s.Require().NoError(probeCtx.Err(), "%s(%d) should return promptly, not hang", name, tc.height)
				s.T().Logf("%s(%d) error: %v", name, tc.height, callErr)
			}

			probe("GetByHeight", func(c context.Context) error {
				_, err := bridgeClient.Header.GetByHeight(c, tc.height)
				return err
			})
			probe("SharesAvailable", func(c context.Context) error {
				return bridgeClient.Share.SharesAvailable(c, tc.height)
			})
		})
	}
}

func (s *E2ESanityTestSuite) createBlobsForSubmission(
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
