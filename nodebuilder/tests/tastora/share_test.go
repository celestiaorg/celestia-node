//go:build integration

package tastora

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"

	libshare "github.com/celestiaorg/go-square/v4/share"

	rpcclient "github.com/celestiaorg/celestia-node/api/rpc/client"
	nodeblob "github.com/celestiaorg/celestia-node/blob"
	"github.com/celestiaorg/celestia-node/header"
	"github.com/celestiaorg/celestia-node/share/shwap"
	"github.com/celestiaorg/celestia-node/state"
)

// ShareTestSuite exercises the Share API on a bridge (local store) and a light node (network
// getters) against the same multi-share blob, in both the original and the parity quadrant.
type ShareTestSuite struct {
	suite.Suite
	framework *Framework

	clients map[string]*rpcclient.Client
	hdr     *header.ExtendedHeader
	blob    *nodeblob.Blob
	// blobShares is the submitted blob split into shares; blobCoords locates its first share.
	blobShares []libshare.Share
	blobCoords shwap.SampleCoords
}

func TestShareTestSuite(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping Share module integration tests in short mode")
	}
	suite.Run(t, &ShareTestSuite{})
}

func (s *ShareTestSuite) SetupSuite() {
	s.framework = NewFramework(s.T(), WithValidators(1), WithBridgeNodes(1), WithLightNodes(1))
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	s.Require().NoError(s.framework.SetupNetwork(ctx))
	s.framework.NewLightNode(ctx)

	bridgeClient := s.framework.GetNodeRPCClient(ctx, s.framework.GetBridgeNodes()[0])
	lightClient := s.framework.GetNodeRPCClient(ctx, s.framework.GetLightNodes()[0])
	s.clients = map[string]*rpcclient.Client{"bridge": bridgeClient, "light": lightClient}

	libBlobs, err := libshare.GenerateV0Blobs([]int{128}, true)
	s.Require().NoError(err)
	nodeBlobs, err := nodeblob.ToNodeBlobs(libBlobs[0])
	s.Require().NoError(err)

	height, err := bridgeClient.Blob.Submit(ctx, nodeBlobs, state.NewTxConfig(state.WithGasPrice(0.1)))
	s.Require().NoError(err, "bridge should submit blob")
	_, err = lightClient.Header.WaitForHeight(ctx, height)
	s.Require().NoError(err, "light should sync the blob height")

	s.blob, err = bridgeClient.Blob.Get(ctx, height, nodeBlobs[0].Namespace(), nodeBlobs[0].Commitment)
	s.Require().NoError(err)
	s.hdr, err = bridgeClient.Header.GetByHeight(ctx, height)
	s.Require().NoError(err)
	s.blobCoords, err = shwap.SampleCoordsFrom1DIndex(s.blob.Index(), len(s.hdr.DAH.RowRoots))
	s.Require().NoError(err)
	s.blobShares, err = nodeblob.BlobsToShares(s.blob)
	s.Require().NoError(err)
}

func (s *ShareTestSuite) TearDownSuite() {
	if s.framework != nil {
		s.framework.Cleanup()
	}
}

// q4Coords points at the last share of the EDS, which lives in the parity quadrant.
func (s *ShareTestSuite) q4Coords() shwap.SampleCoords {
	last := len(s.hdr.DAH.RowRoots) - 1
	return shwap.SampleCoords{Row: last, Col: last}
}

func (s *ShareTestSuite) forEachClient(fn func(ctx context.Context, client *rpcclient.Client)) {
	for name, client := range s.clients {
		s.Run(name, func() {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
			defer cancel()
			fn(ctx, client)
		})
	}
}

func (s *ShareTestSuite) TestSharesAvailable() {
	s.forEachClient(func(ctx context.Context, client *rpcclient.Client) {
		s.Require().NoError(client.Share.SharesAvailable(ctx, s.hdr.Height()))
	})
}

func (s *ShareTestSuite) TestGetShare() {
	s.forEachClient(func(ctx context.Context, client *rpcclient.Client) {
		sh, err := client.Share.GetShare(ctx, s.hdr.Height(), s.blobCoords.Row, s.blobCoords.Col)
		s.Require().NoError(err)
		s.Assert().Equal(s.blobShares[0], sh, "share at the blob's coords should be its first share")

		q4 := s.q4Coords()
		_, err = client.Share.GetShare(ctx, s.hdr.Height(), q4.Row, q4.Col)
		s.Require().NoError(err, "parity quadrant share should be served")
	})
}

func (s *ShareTestSuite) TestGetSamples() {
	s.forEachClient(func(ctx context.Context, client *rpcclient.Client) {
		coords := []shwap.SampleCoords{s.blobCoords, s.q4Coords()}
		samples, err := client.Share.GetSamples(ctx, s.hdr.Height(), coords)
		s.Require().NoError(err)
		s.Require().Len(samples, len(coords))
		for i, c := range coords {
			s.Require().NoError(samples[i].Verify(s.hdr.DAH, c.Row, c.Col), "sample (%d,%d) should verify", c.Row, c.Col)
		}
		s.Assert().Equal(s.blobShares[0], samples[0].Share)
	})
}

func (s *ShareTestSuite) TestGetRow() {
	s.forEachClient(func(ctx context.Context, client *rpcclient.Client) {
		row, err := client.Share.GetRow(ctx, s.hdr.Height(), s.blobCoords.Row)
		s.Require().NoError(err)
		s.Require().NoError(row.Verify(s.hdr.DAH, s.blobCoords.Row))
		shrs, err := row.Shares()
		s.Require().NoError(err)
		s.Assert().Equal(s.blobShares[0], shrs[s.blobCoords.Col])

		q4 := s.q4Coords()
		row, err = client.Share.GetRow(ctx, s.hdr.Height(), q4.Row)
		s.Require().NoError(err)
		s.Require().NoError(row.Verify(s.hdr.DAH, q4.Row), "parity quadrant row should verify")
	})
}

func (s *ShareTestSuite) TestGetEDS() {
	s.forEachClient(func(ctx context.Context, client *rpcclient.Client) {
		eds, err := client.Share.GetEDS(ctx, s.hdr.Height())
		s.Require().NoError(err)
		raw := eds.Row(uint(s.blobCoords.Row))
		sh, err := libshare.FromBytes([][]byte{raw[s.blobCoords.Col]})
		s.Require().NoError(err)
		s.Assert().Equal(s.blobShares[0], sh[0])
	})
}

func (s *ShareTestSuite) TestGetNamespaceData() {
	s.forEachClient(func(ctx context.Context, client *rpcclient.Client) {
		ns := s.blob.Namespace()
		nsData, err := client.Share.GetNamespaceData(ctx, s.hdr.Height(), ns)
		s.Require().NoError(err)
		s.Require().NoError(nsData.Verify(s.hdr.DAH, ns))

		s.assertBlob(nsData.Flatten())
	})
}

func (s *ShareTestSuite) TestGetRange() {
	s.forEachClient(func(ctx context.Context, client *rpcclient.Client) {
		length, err := s.blob.Length()
		s.Require().NoError(err)
		rng, err := client.Share.GetRange(ctx, s.hdr.Height(), s.blob.Index(), s.blob.Index()+length)
		s.Require().NoError(err)
		s.Require().NoError(rng.Verify(s.hdr.DAH.Hash()))

		s.assertBlob(rng.Shares)
	})
}

// TestBlobIncluded verifies a bridge-issued inclusion proof on every node.
func (s *ShareTestSuite) TestBlobIncluded() {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	proof, err := s.clients["bridge"].Blob.GetProof(ctx, s.hdr.Height(), s.blob.Namespace(), s.blob.Commitment)
	s.Require().NoError(err)

	s.forEachClient(func(ctx context.Context, client *rpcclient.Client) {
		got, err := client.Blob.Get(ctx, s.hdr.Height(), s.blob.Namespace(), s.blob.Commitment)
		s.Require().NoError(err)
		s.Assert().Equal(s.blob.Data(), got.Data())

		included, err := client.Blob.Included(ctx, s.hdr.Height(), s.blob.Namespace(), proof, s.blob.Commitment)
		s.Require().NoError(err)
		s.Assert().True(included)
	})
}

func (s *ShareTestSuite) assertBlob(shares []libshare.Share) {
	parsed, err := libshare.ParseBlobs(shares)
	s.Require().NoError(err)
	s.Require().NotEmpty(parsed)
	got, err := nodeblob.ToNodeBlobs(parsed[0])
	s.Require().NoError(err)
	s.Assert().Equal(s.blob.Commitment, got[0].Commitment)
}
