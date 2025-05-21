package e2e

import (
	"bytes"
	"context"
	"fmt"
	rpcclient "github.com/celestiaorg/celestia-node/api/rpc/client"
	nodeblob "github.com/celestiaorg/celestia-node/blob"
	"github.com/celestiaorg/celestia-node/state"
	sdk "github.com/cosmos/cosmos-sdk/types"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	"google.golang.org/grpc"
	"log"
	"testing"
	"time"

	sdkmath "cosmossdk.io/math"
	libshare "github.com/celestiaorg/go-square/v2/share"
	"github.com/celestiaorg/tastora/framework/docker"
	"github.com/celestiaorg/tastora/framework/testutil/sdkacc"
	"github.com/celestiaorg/tastora/framework/testutil/wait"
	"github.com/celestiaorg/tastora/framework/types"
)

func (s *CelestiaTestSuite) TestE2EBlobModule() {
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

	dockerChain, ok := celestia.(*docker.Chain)
	s.Require().True(ok, "celestia is not a docker chain")

	wallet, err := docker.CreateAndFundTestWallet(s.T(), ctx, "test", sdkmath.NewInt(100000000000), dockerChain)
	s.Require().NoError(err, "failed to create test wallet")
	s.Require().NotNil(wallet, "wallet is nil")

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

	p2pInfo, err := bridgeNode.GetP2PInfo(ctx)
	s.Require().NoError(err, "failed to get bridge node p2p info")

	p2pAddr, err := p2pInfo.GetP2PAddress()
	s.Require().NoError(err, "failed to get bridge node p2p address")

	fullNode, err := provider.GetDANode(ctx, types.FullNode)
	s.Require().NoError(err, "failed to get fullnode node")

	err = fullNode.Start(ctx,
		types.WithCoreIP(hostname),
		types.WithGenesisBlockHash(genesisHash),
		types.WithP2PAddress(p2pAddr),
	)

	s.Require().NoError(err, "failed to start bridge node")
	// cleanup resources when the test is done
	t.Cleanup(func() {
		if err := fullNode.Stop(ctx); err != nil {
			t.Logf("Error stopping bridge node: %v", err)
		}
	})

	celestiaHeight, err := celestia.Height(ctx)
	s.Require().NoError(err, "failed to get celestia height")

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

	err = wait.ForDANodeToReachHeight(ctx, fullNode, uint64(celestiaHeight), time.Second*30)
	s.Require().NoError(err, "failed to wait for full node to reach height")

	p2pInfo, err = fullNode.GetP2PInfo(ctx)
	s.Require().NoError(err, "failed to get bridge node p2p info")

	p2pAddr, err = p2pInfo.GetP2PAddress()
	s.Require().NoError(err, "failed to get bridge node p2p address")

	t.Logf("Full node P2P Addr: %s", p2pAddr)

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

	rpcAddr := fullNode.GetHostRPCAddress()
	s.Require().NotEmpty(rpcAddr, "rpc address is empty")

	client, err := rpcclient.NewClient(ctx, "http://"+rpcAddr, "")
	s.Require().NoError(err)
	addr, err := client.State.AccountAddress(ctx)
	s.Require().NoError(err)

	fromAddr, err := sdkacc.AddressFromBech32(wallet.GetFormattedAddress(), "celestia")
	s.Require().NoError(err, "failed to get from address")

	t.Logf("sending funds from %s to %s", fromAddr.String(), addr.String())

	bankSend := banktypes.NewMsgSend(fromAddr, addr.Bytes(), sdk.NewCoins(sdk.NewCoin("utia", sdkmath.NewInt(10000000000))))
	resp, err = celestia.BroadcastMessages(ctx, wallet, bankSend)
	s.Require().NoError(err)
	s.Require().Equal(resp.Code, uint32(0), "resp: %v", resp)

	err = wait.ForBlocks(ctx, 2, celestia)
	s.Require().NoError(err)

	grpcAddr := celestia.GetGRPCAddress()
	bal, err := QueryBalance(ctx, grpcAddr, addr.String())
	s.Require().NoError(err)
	s.Require().Greater(bal.Amount.Int64(), int64(0), "balance is not greater than 0")

	v1Blob, err := libshare.NewV1Blob(
		libshare.MustNewV0Namespace(bytes.Repeat([]byte{5}, libshare.NamespaceVersionZeroIDSize)),
		[]byte("test data"),
		addr.Bytes(),
	)
	s.Require().NoError(err)

	libBlobs0, err := libshare.GenerateV0Blobs([]int{8, 4}, true)
	s.Require().NoError(err)

	libBlobs1, err := libshare.GenerateV0Blobs([]int{4}, false)
	s.Require().NoError(err)

	blobs, err := nodeblob.ToNodeBlobs(append(libBlobs0, libBlobs1...)...)
	s.Require().NoError(err)

	v1, err := nodeblob.ToNodeBlobs(v1Blob)
	s.Require().NoError(err)
	blobs = append(blobs, v1[0])

	_, err = client.Blob.Submit(ctx, blobs, state.NewTxConfig(
		state.WithGas(200_000),
		state.WithGasPrice(5000),
	))
	s.Require().NoError(err)
}

// QueryBalance fetches the balance of a given address and denom from a Cosmos SDK chain via gRPC.
func QueryBalance(ctx context.Context, grpcAddr string, addr string) (sdk.Coin, error) {
	grpcConn, err := grpc.Dial(grpcAddr, grpc.WithInsecure())
	if err != nil {
		log.Fatalf("failed to connect to gRPC: %v", err)
	}
	bankClient := banktypes.NewQueryClient(grpcConn)

	req := &banktypes.QueryBalanceRequest{
		Address: addr,
		Denom:   "utia",
	}

	res, err := bankClient.Balance(ctx, req)
	if err != nil {
		return sdk.Coin{}, fmt.Errorf("failed to query balance: %w", err)
	}

	return *res.Balance, nil
}
