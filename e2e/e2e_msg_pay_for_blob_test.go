package e2e

import (
	"context"
	"encoding/base64"
	"testing"
	"time"

	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/celestiaorg/celestia-app/v4/pkg/appconsts"
	"github.com/celestiaorg/celestia-app/v4/test/util/random"
	blobtypes "github.com/celestiaorg/celestia-app/v4/x/blob/types"
	libshare "github.com/celestiaorg/go-square/v2/share"
	"github.com/celestiaorg/tastora/framework/testutil/sdkacc"
	"github.com/celestiaorg/tastora/framework/testutil/wait"
)

// TestE2EMsgPayForBlob ensures end-to-end functionality of broadcasting and verifying a MsgPayForBlob transaction.
// It sets up and verifies the interaction between Celestia chain nodes, a bridge node, and a light node during the process.
func (s *CelestiaTestSuite) TestE2EMsgPayForBlob() {
	t := s.T()
	if testing.Short() {
		t.Skip("skipping in short mode")
	}

	ctx := context.TODO()
	// wait for some blocks to ensure the bridge node can sync up.
	s.Require().NoError(wait.ForBlocks(ctx, 10, s.celestia))

	celestiaHeight, err := s.celestia.Height(ctx)
	s.Require().NoError(err, "failed to get celestia height")

	wallet := s.CreateTestWallet(ctx, s.celestia, 100000)

	ns := libshare.RandomBlobNamespace()
	signer := wallet.GetFormattedAddress()

	signerAddr, err := sdkacc.AddressFromWallet(wallet)
	s.Require().NoError(err, "failed to get signer address")

	msg, blob := randMsgPayForBlobsWithNamespaceAndSigner(signer, signerAddr, ns, 100)

	resp, err := s.celestia.BroadcastBlobMessage(ctx, wallet, msg, blob)
	s.Require().NoError(err, "failed to broadcast blob message")
	s.Require().NotNil(resp, "broadcast blob message response is nil")
	s.Require().Equal(uint32(0), resp.Code, "expected successful tx broadcast, got error: %s", resp.RawLog)

	err = wait.ForDANodeToReachHeight(ctx, s.bridgeNode, uint64(celestiaHeight), time.Second*30)
	s.Require().NoError(err, "failed to wait for bridge node to reach height")

	s.Require().NoError(wait.ForBlocks(ctx, 10, s.celestia), "failed to wait for blocks")

	celestiaHeight, err = s.celestia.Height(ctx)
	s.Require().NoError(err, "failed to get celestia height")

	err = wait.ForDANodeToReachHeight(ctx, s.lightNode, uint64(celestiaHeight), time.Second*30)
	s.Require().NoError(err, "failed to wait for light node to reach height")

	for i := 1; i < int(celestiaHeight); i++ {
		blobs, err := s.lightNode.GetAllBlobs(ctx, uint64(i), []libshare.Namespace{ns})
		s.Require().NoError(err, "failed to get blobs at height %d", i)

		if len(blobs) > 0 {
			t.Logf("found %d blob(s) at height %d", len(blobs), i)
			s.Require().Len(blobs, 1, "expected one blob at height %d, got %d", i, len(blobs))
			decoded, err := base64.StdEncoding.DecodeString(blobs[0].Data)
			s.Require().NoError(err, "failed to decode blob data at height %d", i)
			s.Require().Equal(blob.Data(), decoded)
			return
		}
	}

	t.Fatalf("failed to find any blobs up to height %d", celestiaHeight)
}

// randMsgPayForBlobsWithNamespaceAndSigner generates a random MsgPayForBlobs message with a random namespace and signer.
func randMsgPayForBlobsWithNamespaceAndSigner(signerStr string, signer sdk.AccAddress, ns libshare.Namespace, size int) (*blobtypes.MsgPayForBlobs, *libshare.Blob) {
	blob, err := blobtypes.NewV1Blob(ns, random.Bytes(size), signer)
	if err != nil {
		panic(err)
	}
	msg, err := blobtypes.NewMsgPayForBlobs(
		signerStr,
		appconsts.LatestVersion,
		blob,
	)
	if err != nil {
		panic(err)
	}
	return msg, blob
}
