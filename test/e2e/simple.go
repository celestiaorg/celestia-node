package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/celestiaorg/celestia-app/v3/app"
	"github.com/celestiaorg/celestia-app/v3/test/e2e/testnet"
	"github.com/celestiaorg/celestia-node/test/e2e/prometheus"
	nodeTestnet "github.com/celestiaorg/celestia-node/test/e2e/testnet"
	"github.com/celestiaorg/knuu/pkg/knuu"
)

const (
	appVersion           = "v3.0.2"
	nodeVersion          = "v0.20.4"
	timeFormat           = "20060102_150405"
	heightMetricsName    = "hdr_store_head_height_gauge"
	instanceMetricName   = "exported_instance"
	queryTxCountInterval = 10 * time.Second
	queryTxCountTimeout  = 3 * time.Minute

	metricRetryInterval = 5 * time.Second
	metricRetryTimeout  = 30 * time.Second
)

func E2ESimple(logger *log.Logger) error {
	const (
		testName                = "E2ESimple"
		validatorsCount         = 4
		bootstrapperBridgeCount = 1
		bootstrapperFullCount   = 1
		bridgeCount             = 1
		fullCount               = 1
		lightCount              = 1
		expectedTxs             = 10
	)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Logging test information
	logger.Printf("Running test: %s, app_version: %s, node_version: %s, validators_count: %d, bootstrapper_bridge_count: %d, bootstrapper_full_count: %d, bridge_count: %d, full_count: %d, light_count: %d", testName, appVersion, nodeVersion, validatorsCount, bootstrapperBridgeCount, bootstrapperFullCount, bridgeCount, fullCount, lightCount)

	// Initializing Kubernetes and Minio clients
	scope := fmt.Sprintf("%s-%s", testName, time.Now().Format(timeFormat))

	// Initializing Knuu
	kn, err := knuu.New(ctx, knuu.Options{
		Scope:        scope,
		ProxyEnabled: true,
	})
	testnet.NoError("failed to initialize knuu", err)
	kn.HandleStopSignal(ctx)
	logger.Printf("Knuu initialized, scope: %s", kn.Scope)

	// Creating and setting up the testnet
	tn, err := nodeTestnet.NewNodeTestnet(ctx, logger, kn, testnet.Options{})
	testnet.NoError("failed to create testnet", err)
	defer tn.Cleanup(ctx)

	tn.SetConsensusParams(app.DefaultInitialConsensusParams())

	// Creating genesis nodes
	logger.Println("Creating testnet validators")
	testnet.NoError("failed to create genesis nodes", tn.CreateGenesisNodes(ctx, validatorsCount, appVersion, 10000000, 0, testnet.DefaultResources, true))

	// Creating transaction simulator
	logger.Println("Creating txsim")
	endpoints, err := tn.RemoteGRPCEndpoints(ctx)
	testnet.NoError("failed to get remote gRPC endpoints", err)
	err = tn.CreateTxClient(ctx, "txsim", testnet.TxsimVersion, 1, "100-2000", 100, testnet.DefaultResources, endpoints[0], nil)
	testnet.NoError("failed to create tx client", err)

	// Setting up and starting the testnet
	logger.Println("Setting up testnet")
	testnet.NoError("failed to setup testnet", tn.Setup(ctx))

	logger.Println("Starting testnet")
	testnet.NoError("failed to start testnet", tn.Start(ctx))

	// Retrieving chain ID and genesis hash
	chainID := tn.ChainID()
	genesisHash, err := tn.GenesisHash(ctx)
	testnet.NoError("failed to get genesis hash", err)
	coreIP := tn.Node(0).Instance.Network().HostName()

	// Creating bootstrapper nodes
	testnet.NoError("failed to create bootstrapper bridge nodes",
		tn.CreateBridgeNodes(ctx, bootstrapperBridgeCount, nodeVersion, chainID, genesisHash, coreIP, true, true, nodeTestnet.DefaultBridgeResources),
	)
	testnet.NoError("failed to create bootstrapper full nodes",
		tn.CreateFullNodes(ctx, bootstrapperFullCount, nodeVersion, chainID, genesisHash, coreIP, true, true, nodeTestnet.DefaultFullResources),
	)

	// Creating other nodes
	testnet.NoError("failed to create bridge nodes",
		tn.CreateBridgeNodes(ctx, bridgeCount, nodeVersion, chainID, genesisHash, coreIP, false, true, nodeTestnet.DefaultBridgeResources),
	)
	testnet.NoError("failed to create full nodes",
		tn.CreateFullNodes(ctx, fullCount, nodeVersion, chainID, genesisHash, coreIP, false, true, nodeTestnet.DefaultFullResources),
	)
	testnet.NoError("failed to create light nodes",
		tn.CreateLightNodes(ctx, lightCount, nodeVersion, chainID, genesisHash, coreIP, nodeTestnet.DefaultLightResources),
	)

	// Enabling Prometheus
	testnet.NoError("failed to enable prometheus", tn.EnablePrometheus(ctx))

	// Setting up and starting the DA testnet
	logger.Println("Setting up DA testnet")
	testnet.NoError("failed to setup DA testnet", tn.SetupDA(ctx))

	logger.Println("Starting DA testnet")
	testnet.NoError("failed to start DA testnet", tn.StartDA(ctx))

	// Waiting for transactions
	logger.Printf("Waiting for at least %d transactions", expectedTxs)
	waitCtx, waitCancel := context.WithTimeout(ctx, queryTxCountTimeout)
	defer waitCancel()
	err = waitForTxs(waitCtx, tn.Node(0).AddressRPC(), expectedTxs, logger)
	testnet.NoError("failed to wait for transactions", err)

	// Retrieving consensus height
	client, err := tn.Node(0).Client()
	testnet.NoError("failed to get client", err)
	consensusHeight, err := getHeight(ctx, client, 10*time.Second)
	testnet.NoError("failed to get consensus height", err)
	logger.Printf("Consensus height: %d", consensusHeight)

	// Waiting for Prometheus metrics
	logger.Println("Waiting for prometheus metrics to be scraped from the DA nodes")
	for _, n := range tn.DaNodes() {
		nodeID, err := n.GetNodeID(ctx)
		testnet.NoError("failed to get node ID", err)
		if nodeID == "" {
			return fmt.Errorf("node ID is empty")
		}

		// Retry for a maximum of 1 minute, checking every 10 seconds
		retryCtx, cancel := context.WithTimeout(ctx, 1*time.Minute)
		defer cancel()

		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()

		success := false
		for !success {
			select {
			case <-retryCtx.Done():
				testnet.NoError("failed to get metric within 1 minute", fmt.Errorf("timeout reached"))
				return fmt.Errorf("failed to get metric within 1 minute")
			case <-ticker.C:
				value, err := tn.Prometheus.GetMetric(prometheus.MetricFilter{
					MetricName: heightMetricsName,
					Labels:     map[string]string{instanceMetricName: nodeID},
				})
				if err == nil {
					logger.Printf("node: %s, metric value: %v", n.Name, value)
					if int(value) >= int(consensusHeight) {
						success = true
						break
					}
				}
			}
		}
	}

	return nil
}
