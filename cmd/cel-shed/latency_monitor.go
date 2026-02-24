package main

import (
	"context"
	"fmt"
	"math/rand"
	"time"

	"github.com/cosmos/cosmos-sdk/crypto/keyring"
	"github.com/spf13/cobra"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	"go.opentelemetry.io/otel/metric"

	libshare "github.com/celestiaorg/go-square/v3/share"

	"github.com/celestiaorg/celestia-node/api/client"
	"github.com/celestiaorg/celestia-node/blob"
	"github.com/celestiaorg/celestia-node/libs/utils"
	"github.com/celestiaorg/celestia-node/nodebuilder/p2p"
)

var runLatencyMonitorCmd = &cobra.Command{
	Use:   "latency-monitor",
	Short: "Runs blob module submission + retrieval latency monitor for celestia-node..",
	RunE: func(cmd *cobra.Command, _ []string) error {
		bridgeAddr, _ := cmd.Flags().GetString("bridge.addr")
		bridgeToken, _ := cmd.Flags().GetString("bridge.token")
		coreGRPCAddr, _ := cmd.Flags().GetString("core.grpc")
		keyName, _ := cmd.Flags().GetString("key.name")
		keyPath, _ := cmd.Flags().GetString("key.path")
		network, _ := cmd.Flags().GetString("network")
		metricsEndpoint, _ := cmd.Flags().GetString("metrics.endpoint")

		cli, signerAddr, err := buildClient(cmd.Context(), bridgeAddr, bridgeToken, coreGRPCAddr, keyName, keyPath, network)
		if err != nil {
			return err
		}
		defer cli.Close()

		fmt.Printf("\nConnected to network: %s\n", network)

		name := fmt.Sprintf("%s/%s", network, signerAddr)
		shutdown, latencyMetrics, err := initializeMetrics(cmd.Context(), metricsEndpoint, name)
		if err != nil {
			return fmt.Errorf("failed to initialize metrics: %w", err)
		}
		defer func() {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			if err := shutdown(ctx); err != nil {
				fmt.Printf("Error shutting down metrics: %v\n", err)
			}
		}()

		fmt.Printf("Metrics initialized, exporting to: %s\n", metricsEndpoint)

		runLatencyMonitor(cmd.Context(), cli, latencyMetrics)
		return nil
	},
}

func init() {
	flags := runLatencyMonitorCmd.Flags()
	flags.String("bridge.addr", "", "address of the bridge node (e.g. http://localhost:26658)")
	flags.String("bridge.token", "", "auth token for the bridge node")
	flags.String("core.grpc", "", "address of the core gRPC server (e.g. localhost:9090)")
	flags.String("key.name", "", "name of the key to use for signing")
	flags.String("key.path", "", "path to the keyring directory")
	flags.String("network", "", "network name (e.g. arabica-11)")
	flags.String("metrics.endpoint", "", "OTLP metrics endpoint")

	_ = runLatencyMonitorCmd.MarkFlagRequired("bridge.addr")
	_ = runLatencyMonitorCmd.MarkFlagRequired("bridge.token")
	_ = runLatencyMonitorCmd.MarkFlagRequired("core.grpc")
	_ = runLatencyMonitorCmd.MarkFlagRequired("key.name")
	_ = runLatencyMonitorCmd.MarkFlagRequired("key.path")
	_ = runLatencyMonitorCmd.MarkFlagRequired("network")
	_ = runLatencyMonitorCmd.MarkFlagRequired("metrics.endpoint")
}

func buildClient(
	ctx context.Context,
	bridgeAddr,
	bridgeToken,
	coreGRPCAddr,
	keyName,
	keyPath,
	network string,
) (*client.Client, string, error) {
	kr, err := client.KeyringWithNewKey(client.KeyringConfig{
		KeyName:     keyName,
		BackendName: keyring.BackendTest,
	}, keyPath)
	if err != nil {
		return nil, "", fmt.Errorf("failed to initialize keyring: %w", err)
	}

	keyInfo, err := kr.Key(keyName)
	if err != nil {
		return nil, "", fmt.Errorf("failed to get key info: %w", err)
	}
	addr, err := keyInfo.GetAddress()
	if err != nil {
		return nil, "", fmt.Errorf("failed to get address from key: %w", err)
	}
	fmt.Printf("\nSubmitting blobs with address: %s\n", addr.String())

	cfg := client.Config{
		ReadConfig: client.ReadConfig{
			BridgeDAAddr: bridgeAddr,
			DAAuthToken:  bridgeToken,
		},
		SubmitConfig: client.SubmitConfig{
			DefaultKeyName: keyName,
			Network:        p2p.Network(network),
			CoreGRPCConfig: client.CoreGRPCConfig{
				Addr: coreGRPCAddr,
			},
		},
	}

	cl, err := client.New(ctx, cfg, kr)
	if err != nil {
		return nil, "", err
	}
	return cl, addr.String(), nil
}

// latencyMetrics holds the metrics for tracking submission and retrieval latency
type latencyMetrics struct {
	submitLatency       metric.Float64Histogram
	retrieveLatency     metric.Float64Histogram
	submitFailedTotal   metric.Int64Counter
	retrieveFailedTotal metric.Int64Counter
}

// initializeMetrics sets up the OTLP metrics exporter and creates latency metrics
func initializeMetrics(
	ctx context.Context,
	endpoint,
	name string,
) (func(context.Context) error, *latencyMetrics, error) {
	cfg := utils.MetricProviderConfig{
		ServiceNamespace:  "latency-monitor",
		ServiceName:       "cel-shed",
		ServiceInstanceID: name,
		Interval:          10 * time.Second,
		OTLPOptions:       []otlpmetrichttp.Option{otlpmetrichttp.WithEndpoint(endpoint)},
	}

	provider, err := utils.NewMetricProvider(ctx, cfg)
	if err != nil {
		return nil, nil, err
	}

	otel.SetMeterProvider(provider)

	// Create meter and metrics
	meter := otel.Meter("latency-monitor")

	submitLatency, err := meter.Float64Histogram(
		"blob_submit_latency_seconds",
		metric.WithDescription("Latency of blob submission operations in s"),
		metric.WithUnit("s"),
	)
	if err != nil {
		return nil, nil, fmt.Errorf("creating submit latency histogram: %w", err)
	}

	retrieveLatency, err := meter.Float64Histogram(
		"blob_retrieve_latency_seconds",
		metric.WithDescription("Latency of blob retrieval operations in s"),
		metric.WithUnit("s"),
	)
	if err != nil {
		return nil, nil, fmt.Errorf("creating retrieve latency histogram: %w", err)
	}

	submitTotal, err := meter.Int64Counter(
		"blob_failed_submit_total",
		metric.WithDescription("Total number of blob failed submission attempts"),
	)
	if err != nil {
		return nil, nil, fmt.Errorf("creating submit total counter: %w", err)
	}

	retrieveTotal, err := meter.Int64Counter(
		"blob_failed_retrieve_total",
		metric.WithDescription("Total number of blob failed retrieval attempts"),
	)
	if err != nil {
		return nil, nil, fmt.Errorf("creating retrieve total counter: %w", err)
	}

	metrics := &latencyMetrics{
		submitLatency:       submitLatency,
		retrieveLatency:     retrieveLatency,
		submitFailedTotal:   submitTotal,
		retrieveFailedTotal: retrieveTotal,
	}

	shutdown := func(ctx context.Context) error {
		return provider.Shutdown(ctx)
	}

	return shutdown, metrics, nil
}

func runLatencyMonitor(ctx context.Context, cli *client.Client, metrics *latencyMetrics) {
	ns := libshare.RandomBlobNamespace()
	fmt.Println("\nUsing namespace:  ", ns.String())

	fmt.Println("\nGenerating blobs...")

	const numBlobs = 10
	libBlobs := make([]*libshare.Blob, numBlobs)
	for i := 0; i < numBlobs; i++ {
		generated, err := libshare.GenerateV0Blobs([]int{16}, true) // TODO @renaynay: variable size
		if err != nil {
			panic(fmt.Sprintf("failed to generate blob %d: %v", i, err))
		}

		libBlobs[i] = generated[0]
		fmt.Printf("Generated blob %d, actual data length: %d bytes\n", i, len(generated[0].Data()))
	}

	blobs, err := blob.ToNodeBlobs(libBlobs...)
	if err != nil {
		panic(fmt.Sprintf("failed to convert blobs: %s", err.Error()))
	}

	for {
		select {
		case <-ctx.Done():
			return
		default:
			randBlob := blobs[rand.Intn(len(blobs))] //nolint:gosec

			fmt.Printf("\nSubmitting blob of size %d bytes...", len(randBlob.Data()))

			operationCtx, cancel := context.WithTimeout(ctx, time.Minute)
			start := time.Now()
			height, err := cli.Blob.Submit(operationCtx, []*blob.Blob{randBlob}, &blob.SubmitOptions{})
			submitDuration := time.Since(start)
			cancel()
			if err != nil {
				fmt.Println("failed to submit blob: ", err)
				metrics.submitFailedTotal.Add(ctx, 1)
				continue
			}

			fmt.Printf("\nSubmitted blob of size %d bytes at height %d\n", len(randBlob.Data()), height)
			fmt.Printf("Submission latency: %f s\n", submitDuration.Seconds())
			metrics.submitLatency.Record(ctx, submitDuration.Seconds())

			operationCtx, cancel = context.WithTimeout(ctx, time.Minute)
			start = time.Now()
			_, err = cli.Blob.Get(operationCtx, height, randBlob.Namespace(), randBlob.Commitment)
			retrieveDuration := time.Since(start)
			cancel()

			// Record retrieve metrics
			if err != nil {
				fmt.Println("failed to retrieve blob: ", err)
				metrics.retrieveFailedTotal.Add(ctx, 1)
				continue
			}

			fmt.Printf("\nGot blob of size %d bytes from height %d\n", len(randBlob.Data()), height)
			fmt.Printf("Retrieval latency: %f s\n", retrieveDuration.Seconds())
			metrics.retrieveLatency.Record(ctx, retrieveDuration.Seconds())
		}
	}
}
