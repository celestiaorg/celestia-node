package main

import (
	"context"
	"fmt"
	"math/rand"
	"time"

	libshare "github.com/celestiaorg/go-square/v3/share"
	"github.com/spf13/cobra"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	"go.opentelemetry.io/otel/metric"
	sdk "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	semconv "go.opentelemetry.io/otel/semconv/v1.11.0"

	"github.com/celestiaorg/celestia-node/api/rpc/client"
	"github.com/celestiaorg/celestia-node/blob"
)

var runLatencyMonitorCmd = &cobra.Command{
	Use:   "latency-monitor <ip> <port> <token> <metrics_endpoint>",
	Short: "Runs blob module submission + retrieval latency monitor for celestia-node..",
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) != 4 {
			return fmt.Errorf("must provide core.ip, core.port, auth token and metrics.endpoint, only "+
				"got %d arguments", len(args))
		}

		cli, err := buildClient(cmd.Context(), args[0], args[1], args[2])
		if err != nil {
			return err
		}

		info, err := cli.P2P.Info(cmd.Context())
		if err != nil {
			return fmt.Errorf("failed to get node info: %w", err)
		}
		fmt.Printf("\nConnected to node with ID: %s\n", info.ID)

		// Initialize metrics
		metricsEndpoint := args[3]
		shutdown, latencyMetrics, err := initializeMetrics(cmd.Context(), metricsEndpoint, info.ID.String())
		if err != nil {
			return fmt.Errorf("failed to initialize metrics: %w", err)
		}
		defer func() {
			if err := shutdown(cmd.Context()); err != nil {
				fmt.Printf("Error shutting down metrics: %v\n", err)
			}
		}()

		fmt.Printf("Metrics initialized, exporting to: %s\n", metricsEndpoint)

		runLatencyMonitor(cmd.Context(), cli, latencyMetrics)
		return nil
	},
}

func buildClient(ctx context.Context, ip, port, token string) (*client.Client, error) {
	addr := fmt.Sprintf("http://%s:%s", ip, port)

	return client.NewClient(ctx, addr, token)
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
	peerID string,
) (func(context.Context) error, *latencyMetrics, error) {
	opts := []otlpmetrichttp.Option{
		otlpmetrichttp.WithCompression(otlpmetrichttp.GzipCompression),
		otlpmetrichttp.WithEndpoint(endpoint),
		// Using secure HTTPS connection by default (omit WithInsecure)
	}

	exp, err := otlpmetrichttp.New(ctx, opts...)
	if err != nil {
		return nil, nil, fmt.Errorf("creating OTLP metric exporter: %w", err)
	}

	provider := sdk.NewMeterProvider(
		sdk.WithReader(
			sdk.NewPeriodicReader(exp,
				sdk.WithTimeout(10*time.Second),
				sdk.WithInterval(10*time.Second))),
		sdk.WithResource(
			resource.NewWithAttributes(
				semconv.SchemaURL,
				semconv.ServiceNamespaceKey.String("latency-monitor"),
				semconv.ServiceNameKey.String("cel-shed"),
				semconv.ServiceInstanceIDKey.String(peerID),
			)))

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
