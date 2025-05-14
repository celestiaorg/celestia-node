package e2e

import (
	"context"
	sdkmath "cosmossdk.io/math"
	"encoding/base64"
	"github.com/celestiaorg/celestia-app/v4/pkg/appconsts"
	"github.com/celestiaorg/celestia-app/v4/test/util/random"
	blobtypes "github.com/celestiaorg/celestia-app/v4/x/blob/types"
	libshare "github.com/celestiaorg/go-square/v2/share"
	"github.com/celestiaorg/tastora/framework/docker"
	"github.com/celestiaorg/tastora/framework/testutil/sdkacc"
	"github.com/celestiaorg/tastora/framework/types"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/celestiaorg/tastora/framework/testutil/wait"
	"testing"
	"time"
)

// TestE2EMsgPayForBlob ensures end-to-end functionality of broadcasting and verifying a MsgPayForBlob transaction.
// It sets up and verifies the interaction between Celestia chain nodes, a bridge node, and a light node during the process.
func (s *CelestiaTestSuite) TestE2EMsgPayForBlob() {
	t := s.T()
	if testing.Short() {
		t.Skip("skipping in short mode")
	}

	ctx := context.TODO()
	provider := s.CreateDockerProvider()

	celestia, err := provider.GetChain(ctx)
	s.Require().NoError(err)

	err = celestia.Start(ctx)
	s.Require().NoError(err)

	// cleanup resources when the test is done
	t.Cleanup(func() {
		if err := celestia.Stop(ctx); err != nil {
			t.Logf("Error stopping chain: %v", err)
		}
	})

	// verify the chain is producing blocks
	height, err := celestia.Height(ctx)
	s.Require().NoError(err)
	s.Require().Greater(height, int64(0))

	// wait for some blocks to ensure the bridge node can sync up.
	s.Require().NoError(wait.ForBlocks(ctx, 10, celestia))

	chainNode := celestia.GetNodes()[0]
	genesisHash := s.getGenesisHash(ctx, chainNode)
	s.Require().NotEmpty(genesisHash, "genesis hash is empty")

	bridgeNode, err := provider.GetDANode(ctx, types.BridgeNode)
	s.Require().NoError(err, "failed to get bridge node")

	hostname, err := chainNode.GetInternalHostName(ctx)
	s.Require().NoError(err, "failed to get internal hostname")

	err = bridgeNode.Start(ctx,
		types.WithCoreIP(hostname),
		types.WithGenesisBlockHash(genesisHash),
	)

	s.Require().NoError(err, "failed to start bridge node")
	// cleanup resources when the test is done
	t.Cleanup(func() {
		if err := bridgeNode.Stop(ctx); err != nil {
			t.Logf("Error stopping bridge node: %v", err)
		}
	})

	celestiaHeight, err := celestia.Height(ctx)
	s.Require().NoError(err, "failed to get celestia height")

	dockerChain, ok := celestia.(*docker.Chain)
	s.Require().True(ok, "celestia is not a docker chain")

	wallet, err := docker.CreateAndFundTestWallet(s.T(), ctx, "test", sdkmath.NewInt(100000), dockerChain)
	s.Require().NoError(err, "failed to create test wallet")
	s.Require().NotNil(wallet, "wallet is nil")

	ns := libshare.RandomBlobNamespace()
	signer := wallet.GetFormattedAddress()

	signerAddr, err := sdkacc.AddressFromBech32(signer, "celestia")
	s.Require().NoError(err, "failed to get signer address")

	msg, blob := randMsgPayForBlobsWithNamespaceAndSigner(signer, signerAddr, ns, 100)

	resp, err := celestia.BroadcastBlobMessage(ctx, wallet, msg, blob)
	s.Require().NoError(err, "failed to broadcast blob message")
	s.Require().NotNil(resp, "broadcast blob message response is nil")
	s.Require().Equal(uint32(0), resp.Code, "expected successful tx broadcast, got error: %s", resp.RawLog)

	err = wait.ForDANodeToReachHeight(ctx, bridgeNode, uint64(celestiaHeight), time.Second*30)
	s.Require().NoError(err, "failed to wait for bridge node to reach height")

	p2pInfo, err := bridgeNode.GetP2PInfo(ctx)
	s.Require().NoError(err, "failed to get bridge node p2p info")

	p2pAddr, err := p2pInfo.GetP2PAddress()
	s.Require().NoError(err, "failed to get bridge node p2p address")

	t.Logf("P2P Addr: %s", p2pAddr)

	lightNode, err := provider.GetDANode(ctx, types.LightNode)
	s.Require().NoError(err, "failed to get light node")

	err = lightNode.Start(ctx,
		types.WithP2PAddress(p2pAddr),
		types.WithCoreIP(hostname),
		types.WithGenesisBlockHash(genesisHash),
	)
	s.Require().NoError(err, "failed to start light node")
	// cleanup resources when the test is done
	t.Cleanup(func() {
		if err := lightNode.Stop(ctx); err != nil {
			t.Logf("Error stopping light node: %v", err)
		}
	})

	s.Require().NoError(wait.ForBlocks(ctx, 10, celestia), "failed to wait for blocks")

	celestiaHeight, err = celestia.Height(ctx)
	s.Require().NoError(err, "failed to get celestia height")

	err = wait.ForDANodeToReachHeight(ctx, lightNode, uint64(celestiaHeight), time.Second*30)
	s.Require().NoError(err, "failed to wait for light node to reach height")

	for i := 1; i < int(celestiaHeight); i++ {
		blobs, err := lightNode.GetAllBlobs(ctx, uint64(i), []libshare.Namespace{ns})
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
