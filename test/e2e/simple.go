package main

import (
	"context"
	"fmt"
	"time"

	"github.com/celestiaorg/celestia-app/v3/app"
	"github.com/celestiaorg/celestia-app/v3/test/e2e/testnet"
	"github.com/celestiaorg/celestia-app/v3/test/util/testnode"
	"github.com/celestiaorg/celestia-node/test/e2e/prometheus"
	nodeTestnet "github.com/celestiaorg/celestia-node/test/e2e/testnet"
	"github.com/celestiaorg/knuu/pkg/k8s"
	"github.com/celestiaorg/knuu/pkg/knuu"
	"github.com/celestiaorg/knuu/pkg/minio"

	"github.com/sirupsen/logrus"
)

const (
	// appVersion = "v2.2.0"
	// appVersion                    = "206b96c"
	appVersion           = "v3.0.0-rc0"
	nodeVersion          = "v0.17.2"
	timeFormat           = "20060102_150405"
	buildInfoMetricName  = "build_info"
	instanceMetricName   = "exported_instance"
	queryTxCountInterval = 10 * time.Second
	queryTxCountTimeout  = 3 * time.Minute

	metricRetryInterval = 5 * time.Second
	metricRetryTimeout  = 30 * time.Second
)

// This test runs a simple testnet with 4 validators. It submits both MsgPayForBlobs
// and MsgSends over 30 seconds and then asserts that at least 10 transactions were
// committed.
func E2ESimple(logger *logrus.Logger) error {
	const (
		testName        = "E2ESimple"
		validatorsCount = 4
		expectedTxs     = 10
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

	tn, err := nodeTestnet.NewNodeTestnet(ctx, kn, testnet.Options{})
	testnet.NoError("failed to create testnet", err)
	defer tn.NodeCleanup(ctx)

	tn.SetConsensusParams(app.DefaultInitialConsensusParams())

	logger.Info("Creating testnet validators")
	testnet.NoError("failed to create genesis nodes", tn.CreateGenesisNodes(ctx, validatorsCount, appVersion, 10000000, 0, testnet.DefaultResources, true))

	logger.Info("Creating txsim")
	endpoints, err := tn.RemoteGRPCEndpoints(ctx)
	testnet.NoError("failed to get remote gRPC endpoints", err)
	err = tn.CreateTxClient(ctx, "txsim", testnet.TxsimVersion, 1, "100-2000", 100, testnet.DefaultResources, endpoints[0], nil)
	testnet.NoError("failed to create tx client", err)

	logger.Info("Setting up testnets")
	testnet.NoError("failed to setup testnets", tn.Setup(ctx))

	logger.Info("Starting testnets")
	testnet.NoError("failed to start testnets", tn.Start(ctx))

	logger.Infof("Waiting for at least %d transactions", expectedTxs)
	waitCtx, waitCancel := context.WithTimeout(ctx, queryTxCountTimeout)
	defer waitCancel()
	err = waitForTxs(waitCtx, tn.Node(0).AddressRPC(), expectedTxs, logger)
	testnet.NoError("failed to wait for transactions", err)

	// FIXME: If you deploy more than one node of the same type, the keys will be the same
	err = tn.CreateAndStartBridgeNodes(ctx, 1,
		nodeTestnet.InstanceOptions{
			InstanceName: "bridge",
			Version:      nodeVersion,
			Resources:    nodeTestnet.DefaultBridgeResources,
		})
	testnet.NoError("failed to create and start bridge node", err)

	err = tn.CreateAndStartFullNodes(ctx, 1,
		nodeTestnet.InstanceOptions{
			InstanceName: "full",
			Version:      nodeVersion,
			Resources:    nodeTestnet.DefaultFullResources,
		})
	testnet.NoError("failed to create and start full node", err)

	err = tn.CreateAndStartLightNodes(ctx, 1,
		nodeTestnet.InstanceOptions{
			InstanceName: "light",
			Version:      nodeVersion,
			Resources:    nodeTestnet.DefaultLightResources,
		})
	testnet.NoError("failed to create and start light node", err)

	// The prometheus instance needs to be started after the nodes are created
	// because all nodes exporters are added to it before it starts
	err = tn.Prometheus.Start(ctx)
	testnet.NoError("failed to start prometheus", err)

	logger.Info("Waiting for prometheus metrics to be scraped from the DA nodes")
	for _, n := range tn.DaNodes() {
		nodeID, err := n.GetNodeID(ctx)
		testnet.NoError("failed to get node ID", err)
		if nodeID == "" {
			return fmt.Errorf("node ID is empty")
		}

		err = retryEventually(ctx, func() error {
			value, err := tn.Prometheus.GetMetric(prometheus.MetricFilter{
				MetricName: buildInfoMetricName,
				Labels:     map[string]string{instanceMetricName: nodeID},
			})
			if err == nil {
				logger.Infof("node: %s, metric value: %v", n.Name, value)
			}
			return err
		}, metricRetryInterval, metricRetryTimeout)

		testnet.NoError("failed to get metric", err)
	}

	return nil
}

func queryTxCount(ctx context.Context, rpcAddr string) (int, error) {
	blockchain, err := testnode.ReadBlockchain(ctx, rpcAddr)
	if err != nil {
		return 0, err
	}

	totalTxs := 0
	for _, block := range blockchain {
		totalTxs += len(block.Data.Txs)
	}
	return totalTxs, nil
}

func waitForTxs(ctx context.Context, rpcAddr string, expectedTxs int, logger *logrus.Logger) error {
	ticker := time.NewTicker(queryTxCountInterval)
	defer ticker.Stop()

	for range ticker.C {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		totalTxs, err := queryTxCount(ctx, rpcAddr)
		if err != nil {
			return err
		}

		if totalTxs >= expectedTxs {
			logger.Infof("Found %d transactions", totalTxs)
			return nil
		}
		logger.Debugf("Waiting for at least %d transactions, got %d so far", expectedTxs, totalTxs)
	}
	return fmt.Errorf("unknown error")
}

func retryEventually(ctx context.Context, fn func() error, interval time.Duration, timeout time.Duration) error {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	var err error
	for range ticker.C {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		if err = fn(); err == nil {
			return nil
		}
	}
	return err
}
