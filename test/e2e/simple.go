package main

import (
	"context"
	"fmt"
	"time"

	"github.com/celestiaorg/celestia-app/v3/app"
	"github.com/celestiaorg/celestia-app/v3/test/e2e/testnet"
	"github.com/celestiaorg/celestia-app/v3/test/util/testnode"
	"github.com/celestiaorg/celestia-node/nodebuilder/node"
	nodeTestnet "github.com/celestiaorg/celestia-node/test/e2e/testnet"
	"github.com/celestiaorg/knuu/pkg/k8s"
	"github.com/celestiaorg/knuu/pkg/knuu"
	"github.com/celestiaorg/knuu/pkg/minio"

	"github.com/sirupsen/logrus"
)

const (
	// appVersion  = "v2.2.0"
	appVersion  = "206b96c"
	nodeVersion = "v0.17.1"
	timeFormat  = "20060102_150405"
)

// This test runs a simple testnet with 4 validators. It submits both MsgPayForBlobs
// and MsgSends over 30 seconds and then asserts that at least 10 transactions were
// committed.
func E2ESimple(logger *logrus.Logger) error {
	const (
		testName        = "E2ESimple"
		validatorsCount = 4
	)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	logger.WithFields(logrus.Fields{
		"test_name":        testName,
		"app_version":      appVersion,
		"node_version":     nodeVersion,
		"validators_count": validatorsCount,
	}).Info("Running test")

	scope := fmt.Sprintf("%s-%s", testName, time.Now().Format(timeFormat))
	k8sClient, err := k8s.NewClient(ctx, scope, logger)
	testnet.NoError("failed to create k8s client", err)

	minioClient, err := minio.New(ctx, k8sClient, logger)
	testnet.NoError("failed to create minio client", err)

	kn, err := knuu.New(ctx, knuu.Options{
		ProxyEnabled: true,
		K8sClient:    k8sClient,
		MinioClient:  minioClient,
	})
	testnet.NoError("failed to initialize knuu", err)
	kn.HandleStopSignal(ctx)
	logger.WithField("scope", kn.Scope).Info("Knuu initialized")

	testNet, err := nodeTestnet.NewNodeTestnet(ctx, kn, testnet.Options{})
	testnet.NoError("failed to create testnet", err)

	testNet.SetConsensusParams(app.DefaultInitialConsensusParams())

	defer testNet.NodeCleanup(ctx)

	logger.Info("Creating testnet validators")
	testnet.NoError("failed to create genesis nodes", testNet.CreateGenesisNodes(ctx, validatorsCount, appVersion, 10000000, 0, testnet.DefaultResources, true))

	logger.Info("Creating txsim")
	endpoints, err := testNet.RemoteGRPCEndpoints(ctx)
	testnet.NoError("failed to get remote gRPC endpoints", err)
	err = testNet.CreateTxClient(ctx, "txsim", testnet.TxsimVersion, 1, "100-2000", 100, testnet.DefaultResources, endpoints[0], nil)
	testnet.NoError("failed to create tx client", err)

	logger.Info("Setting up testnets")
	testnet.NoError("failed to setup testnets", testNet.Setup(ctx))

	logger.Info("Starting testnets")
	testnet.NoError("failed to start testnets", testNet.Start(ctx))

	// FIXME: If you deploy more than one node of the same type, the keys will be the same
	err = testNet.CreateAndStartBridgeNodes(ctx, 1,
		nodeTestnet.InstanceOptions{
			InstanceName: "bridge",
			Version:      nodeVersion,
			Resources:    nodeTestnet.DefaultBridgeResources,
		})
	testnet.NoError("failed to create and start bridge node", err)

	err = testNet.CreateAndStartFullNodes(ctx, 1,
		nodeTestnet.InstanceOptions{
			InstanceName: "full",
			Version:      nodeVersion,
			Resources:    nodeTestnet.DefaultFullResources,
		})
	testnet.NoError("failed to create and start full node", err)

	err = testNet.CreateAndStartLightNodes(ctx, 1,
		nodeTestnet.InstanceOptions{
			InstanceName: "light",
			Version:      nodeVersion,
			Resources:    nodeTestnet.DefaultLightResources,
		})
	testnet.NoError("failed to create and start light node", err)

	logger.Info("Waiting for 30 seconds")
	time.Sleep(30 * time.Second)

	logger.Info("Reading blockchain")
	blockchain, err := testnode.ReadBlockchain(ctx, testNet.Node(0).AddressRPC())
	testnet.NoError("failed to read blockchain", err)

	totalTxs := 0
	for _, block := range blockchain {
		totalTxs += len(block.Data.Txs)
	}
	if totalTxs < 10 {
		return fmt.Errorf("expected at least 10 transactions, got %d", totalTxs)
	}

	for _, n := range testNet.DaNodes() {
		if n.Type == node.Full || n.Type == node.Light {
			head, err := n.GetMetric("das_sampled_chain_head")
			testnet.NoError("failed to get metric", err)
			if head < 50 {
				return fmt.Errorf("expected head to be greater than 50, got %f", head)
			}
		}
	}

	return nil
}
