//go:build integration

package tastora

import (
	"bytes"
	"context"
	"time"

	"github.com/celestiaorg/go-square/v4/share"

	rpcclient "github.com/celestiaorg/celestia-node/api/rpc/client"
	nodeblob "github.com/celestiaorg/celestia-node/blob"
	"github.com/celestiaorg/celestia-node/state"
)

// TestBridgeServesSharesOverShrex submits a blob to bridge[0] (A) and exercises the full Share API
// surface against it:
//   - GetShare,
//   - GetEDS,
//   - GetRow,
//   - GetRange,
//   - GetNamespaceData,
//
// proving A serves every share-retrieval path for a real, non-empty square.
func (s *P2PTestSuite) TestBridgeServesSharesOverShrex() {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	bridges := s.framework.GetBridgeNodes()
	s.Require().Len(bridges, 2, "suite must run with two bridge nodes")

	clientA := s.framework.GetNodeRPCClient(ctx, bridges[0])
	clientB := s.framework.GetNodeRPCClient(ctx, bridges[1])

	namespace, err := share.NewV0Namespace(bytes.Repeat([]byte{0x05}, 10))
	s.Require().NoError(err, "should create namespace")

	blobData := []byte("TestBridgeServesSharesOverShrex: share API over the network")
	nodeAddr, err := clientA.State.AccountAddress(ctx)
	s.Require().NoError(err, "should get bridge[0] account address")

	libBlob, err := share.NewV1Blob(namespace, blobData, nodeAddr.Bytes())
	s.Require().NoError(err, "should create libshare blob")

	nodeBlobs, err := nodeblob.ToNodeBlobs(libBlob)
	s.Require().NoError(err, "should convert to node blobs")

	txConfig := state.NewTxConfig(state.WithGas(300_000), state.WithGasPrice(5000))

	height, err := clientA.Blob.Submit(ctx, nodeBlobs, txConfig)
	s.Require().NoError(err, "bridge[0] should submit blob")
	s.Require().NotZero(height, "blob submission should return valid height")

	_, err = clientA.Header.WaitForHeight(ctx, height)
	s.Require().NoError(err, "bridge[0] should reach the submitted height")

	hdr, err := clientA.Header.GetByHeight(ctx, height)
	s.Require().NoError(err, "bridge[0] should serve the header at height %d", height)

	err = clientA.Share.SharesAvailable(ctx, height)
	s.Require().NoError(err, "bridge[0] shares should be available at height %d", height)

	edsA, err := clientA.Share.GetEDS(ctx, height)
	s.Require().NoError(err, "bridge[0] should serve the EDS")
	s.Require().NotNil(edsA, "EDS should not be nil")
	width := int(edsA.Width())
	s.Require().Positive(width, "EDS width should be positive")

	shareA, err := clientA.Share.GetShare(ctx, height, 0, 0)
	s.Require().NoError(err, "bridge[0] should serve share (0,0)")

	row, err := clientA.Share.GetRow(ctx, height, 0)
	s.Require().NoError(err, "bridge[0] should serve row 0")
	rowShares, err := row.Shares()
	s.Require().NoError(err, "row should reconstruct its shares")
	s.Assert().Len(rowShares, width, "row 0 should hold a full EDS-width row of shares")
	s.Require().NoError(row.Verify(hdr.DAH, 0), "bridge[0] invalid row at index 0")

	rng, err := clientA.Share.GetRange(ctx, height, 0, 1)
	s.Require().NoError(err, "bridge[0] should serve a share range")
	s.Require().NotEmpty(rng.Shares, "range should contain shares")
	s.Require().NotNil(rng.Proof, "range should carry an inclusion proof")
	s.Require().NoError(rng.Verify(hdr.DataHash), "range proof should validate against the data root")

	nsDataA, err := clientA.Share.GetNamespaceData(ctx, height, namespace)
	s.Require().NoError(err, "bridge[0] should serve namespace data")
	s.Require().NotEmpty(nsDataA.Flatten(), "namespace data should contain the submitted blob shares")
	s.Require().NoError(nsDataA.Verify(hdr.DAH, namespace), "bridge[0] proof verification failed")

	// same height retrieved on B (peered to A) returns byte-identical shares.
	_, err = clientB.Header.WaitForHeight(ctx, height)
	s.Require().NoError(err, "bridge[1] should reach the submitted height")

	err = clientB.Share.SharesAvailable(ctx, height)
	s.Require().NoError(err, "bridge[1] shares should be available at height %d", height)

	edsB, err := clientB.Share.GetEDS(ctx, height)
	s.Require().NoError(err, "bridge[1] should serve the EDS")
	// the RPC-decoded square loses its NMT constructor, so compare by content, not recomputed roots.
	s.Assert().Equal(edsA.Flattened(), edsB.Flattened(), "both bridges should serve the same square")

	shareB, err := clientB.Share.GetShare(ctx, height, 0, 0)
	s.Require().NoError(err, "bridge[1] should serve share (0,0)")
	s.Assert().Equal(shareA, shareB, "share (0,0) should match across bridges")

	nsDataB, err := clientB.Share.GetNamespaceData(ctx, height, namespace)
	s.Require().NoError(err, "bridge[1] should serve namespace data")
	s.Assert().Equal(nsDataA.Flatten(), nsDataB.Flatten(), "namespace data should match across bridges")
	s.Require().NoError(nsDataB.Verify(hdr.DAH, namespace), "bridge[1] proof verification failed")
}

func (s *P2PTestSuite) TestNoPeerHasData() {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	bridges := s.framework.GetBridgeNodes()
	s.Require().Len(bridges, 2, "suite must run with two bridge nodes")

	clientA := s.framework.GetNodeRPCClient(ctx, bridges[0])
	clientB := s.framework.GetNodeRPCClient(ctx, bridges[1])

	absentNs, err := share.NewV0Namespace(bytes.Repeat([]byte{0xAB}, 10))
	s.Require().NoError(err, "should create absent namespace")

	head, err := clientB.Header.LocalHead(ctx)
	s.Require().NoError(err, "bridge[1] should report its local head")
	height := head.Height()
	_, err = clientA.Header.WaitForHeight(ctx, height)
	s.Require().NoError(err, "bridge[0] should reach height %d", height)

	for _, tc := range []struct {
		name   string
		client *rpcclient.Client
	}{{"bridge[0]", clientA}, {"bridge[1]", clientB}} {
		nsData, err := tc.client.Share.GetNamespaceData(ctx, height, absentNs)
		s.Require().NoError(err, "%s GetNamespaceData should not error for an absent namespace", tc.name)
		s.Assert().True(nsData.IsEmpty(), "%s should return empty namespace data for an absent namespace", tc.name)

		blobs, err := tc.client.Blob.GetAll(ctx, height, []share.Namespace{absentNs})
		s.Require().NoError(err, "%s Blob.GetAll should not error for an absent namespace", tc.name)
		s.Assert().Empty(blobs, "%s Blob.GetAll should return no blobs for an absent namespace", tc.name)

		_, err = tc.client.Blob.Get(ctx, height, absentNs, bytes.Repeat([]byte{0xFF}, 32))
		s.Require().Error(err, "%s Blob.Get should error for an absent blob", tc.name)
		s.Assert().Contains(err.Error(), nodeblob.ErrBlobNotFound.Error(), "%s should report blob-not-found", tc.name)
	}
}
