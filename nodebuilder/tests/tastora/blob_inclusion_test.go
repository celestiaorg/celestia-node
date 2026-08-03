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

// BlobInclusionTestSuite verifies blob inclusion.
type BlobInclusionTestSuite struct {
	suite.Suite
	framework *Framework
}

func TestBlobInclusionTestSuite(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping blob inclusion integration tests in short mode")
	}
	suite.Run(t, &BlobInclusionTestSuite{})
}

func (s *BlobInclusionTestSuite) SetupSuite() {
	s.framework = NewFramework(s.T(), WithValidators(1), WithBridgeNodes(1), WithLightNodes(0))
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	s.Require().NoError(s.framework.SetupNetwork(ctx))
}

func (s *BlobInclusionTestSuite) TearDownSuite() {
	if s.framework != nil {
		s.framework.Cleanup()
	}
}

// TestBlobInclusion submits a blob under a namespace and verifies it is included.
func (s *BlobInclusionTestSuite) TestBlobInclusion() {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	bridge := s.framework.GetBridgeNodes()[0]
	client := s.framework.GetNodeRPCClient(ctx, bridge)

	namespace, err := share.NewV0Namespace(bytes.Repeat([]byte{0x0A}, 10))
	s.Require().NoError(err, "should create namespace")

	blobData := []byte("TestBlobInclusion: included blob")
	nodeBlobs := s.createBlob(ctx, client, namespace, blobData)

	txConfig := state.NewTxConfig(state.WithGas(300_000), state.WithGasPrice(5000))
	height, err := client.Blob.Submit(ctx, nodeBlobs, txConfig)
	s.Require().NoError(err, "should submit blob")
	s.Require().NotZero(height)

	_, err = client.Header.WaitForHeight(ctx, height)
	s.Require().NoError(err, "should reach the blob height")

	commitment := nodeBlobs[0].Commitment

	// The inclusion proof verifies against the committed blob.
	proof, err := client.Blob.GetProof(ctx, height, namespace, commitment)
	s.Require().NoError(err, "should get inclusion proof")
	s.Require().NotNil(proof)

	included, err := client.Blob.Included(ctx, height, namespace, proof, commitment)
	s.Require().NoError(err, "should verify inclusion")
	s.Require().True(included, "blob should be proven included at height %d", height)

	// The blob round-trips through Get.
	got, err := client.Blob.Get(ctx, height, namespace, commitment)
	s.Require().NoError(err, "should get the included blob")
	s.Require().Equal(blobData, bytes.TrimRight(got.Data(), "\x00"), "blob should round-trip")
}

// TestBlobNonInclusion submits a blob under one namespace then asserts a different
// namespace has no blob at that height.
func (s *BlobInclusionTestSuite) TestBlobNonInclusion() {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	bridge := s.framework.GetBridgeNodes()[0]
	client := s.framework.GetNodeRPCClient(ctx, bridge)

	present, err := share.NewV0Namespace(bytes.Repeat([]byte{0x0B}, 10))
	s.Require().NoError(err, "should create present namespace")
	nodeBlobs := s.createBlob(ctx, client, present, []byte("TestBlobNonInclusion: present blob"))

	txConfig := state.NewTxConfig(state.WithGas(300_000), state.WithGasPrice(5000))
	height, err := client.Blob.Submit(ctx, nodeBlobs, txConfig)
	s.Require().NoError(err, "should submit blob")
	s.Require().NotZero(height)

	_, err = client.Header.WaitForHeight(ctx, height)
	s.Require().NoError(err, "should reach the blob height")

	// A different namespace has no blob at that height.
	absent, err := share.NewV0Namespace(bytes.Repeat([]byte{0x0C}, 10))
	s.Require().NoError(err, "should create absent namespace")

	_, err = client.Blob.Get(ctx, height, absent, nodeBlobs[0].Commitment)
	s.Require().ErrorContains(err, nodeblob.ErrBlobNotFound.Error(), "absent namespace should report no blob")

	all, err := client.Blob.GetAll(ctx, height, []share.Namespace{absent})
	if err != nil {
		s.Require().ErrorContains(err, nodeblob.ErrBlobNotFound.Error(),
			"GetAll on an absent namespace should not error otherwise")
	}
	s.Require().Empty(all, "absent namespace should yield no blobs")
}

func (s *BlobInclusionTestSuite) createBlob(
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
