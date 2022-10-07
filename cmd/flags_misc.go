package cmd

import (
	"context"
	"fmt"
	"net/http"
	"net/http/pprof"
	"strings"
	"time"

	logging "github.com/ipfs/go-log/v2"
	"github.com/spf13/cobra"
	flag "github.com/spf13/pflag"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/sdk/resource"
	tracesdk "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.10.0"

	"github.com/celestiaorg/celestia-node/logs"
	"github.com/celestiaorg/celestia-node/nodebuilder"
)

var (
	logLevelFlag        = "log.level"
	logLevelModuleFlag  = "log.level.module"
	pprofFlag           = "pprof"
	tracingFlag         = "tracing"
	tracingEndpointFlag = "tracing.endpoint"
	tracingTlS          = "tracing.tls"
	metricsFlag         = "metrics"
	metricsEndpointFlag = "metrics.endpoint"
	metricsTlS          = "metrics.tls"
)

// MiscFlags gives a set of hardcoded miscellaneous flags.
func MiscFlags() *flag.FlagSet {
	flags := &flag.FlagSet{}

	flags.String(
		logLevelFlag,
		"INFO",
		`DEBUG, INFO, WARN, ERROR, DPANIC, PANIC, FATAL
and their lower-case forms`,
	)

	flags.StringSlice(
		logLevelModuleFlag,
		nil,
		"<module>:<level>, e.g. pubsub:debug",
	)

	flags.Bool(
		pprofFlag,
		false,
		"Enables standard profiling handler (pprof) and exposes the profiles on port 6000",
	)

	flags.Bool(
		tracingFlag,
		false,
		"Enables OTLP tracing with HTTP exporter",
	)

	flags.String(
		tracingEndpointFlag,
		"localhost:4318",
		"Sets HTTP endpoint for OTLP traces to be exported to. Depends on '--tracing'",
	)

	flags.Bool(
		tracingTlS,
		true,
		"Enable TLS connection to OTLP tracing backend",
	)

	flags.Bool(
		metricsFlag,
		false,
		"Enables OTLP metrics with HTTP exporter",
	)

	flags.String(
		metricsEndpointFlag,
		"localhost:4318",
		"Sets HTTP endpoint for OTLP metrics to be exported to. Depends on '--metrics'",
	)

	flags.Bool(
		metricsTlS,
		true,
		"Enable TLS connection to OTLP metric backend",
	)

	return flags
}

// ParseMiscFlags parses miscellaneous flags from the given cmd and applies values to Env.
func ParseMiscFlags(ctx context.Context, cmd *cobra.Command) (context.Context, error) {
	logLevel := cmd.Flag(logLevelFlag).Value.String()
	if logLevel != "" {
		level, err := logging.LevelFromString(logLevel)
		if err != nil {
			return ctx, fmt.Errorf("cmd: while parsing '%s': %w", logLevelFlag, err)
		}

		logs.SetAllLoggers(level)
	}

	logModules, err := cmd.Flags().GetStringSlice(logLevelModuleFlag)
	if err != nil {
		panic(err)
	}
	for _, ll := range logModules {
		params := strings.Split(ll, ":")
		if len(params) != 2 {
			return ctx, fmt.Errorf("cmd: %s arg must be in form <module>:<level>, e.g. pubsub:debug", logLevelModuleFlag)
		}

		err := logging.SetLogLevel(params[0], params[1])
		if err != nil {
			return ctx, err
		}
	}

	ok, err := cmd.Flags().GetBool(pprofFlag)
	if err != nil {
		panic(err)
	}

	if ok {
		// TODO(@Wondertan): Eventually, this should be registered on http server in RPC
		//  by passing the http.Server with preregistered pprof handlers to the node.
		//  Node should not register pprof itself.
		go func() {
			mux := http.NewServeMux()
			mux.HandleFunc("/debug/pprof/", pprof.Index)
			mux.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
			mux.HandleFunc("/debug/pprof/profile", pprof.Profile)
			mux.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
			mux.HandleFunc("/debug/pprof/trace", pprof.Trace)
			srv := http.Server{
				Addr:         "0.0.0.0:6000",
				Handler:      mux,
				ReadTimeout:  5 * time.Second,
				WriteTimeout: 10 * time.Second,
			}

			if err := srv.ListenAndServe(); err != nil {
				log.Fatalw("failed to start pprof server", "err", err)
			} else {
				log.Info("started pprof server on port 6000")
			}
		}()
	}

	ok, err = cmd.Flags().GetBool(tracingFlag)
	if err != nil {
		panic(err)
	}

	if ok {
		opts := []otlptracehttp.Option{
			otlptracehttp.WithCompression(otlptracehttp.GzipCompression),
			otlptracehttp.WithEndpoint(cmd.Flag(tracingEndpointFlag).Value.String()),
		}
		if ok, err := cmd.Flags().GetBool(tracingTlS); err != nil {
			panic(err)
		} else if !ok {
			opts = append(opts, otlptracehttp.WithInsecure())
		}

		exp, err := otlptracehttp.New(cmd.Context(), opts...)
		if err != nil {
			return ctx, err
		}

		tp := tracesdk.NewTracerProvider(
			// Always be sure to batch in production.
			tracesdk.WithBatcher(exp),
			// Record information about this application in a Resource.
			tracesdk.WithResource(resource.NewWithAttributes(
				semconv.SchemaURL,
				semconv.ServiceNameKey.String(fmt.Sprintf("Celestia-%s", NodeType(ctx).String())),
				// TODO(@Wondertan): Versioning: semconv.ServiceVersionKey
			)),
		)
		otel.SetTracerProvider(tp)
	}

	ok, err = cmd.Flags().GetBool(metricsFlag)
	if err != nil {
		panic(err)
	}

	if ok {
		opts := []otlpmetrichttp.Option{
			otlpmetrichttp.WithCompression(otlpmetrichttp.GzipCompression),
			otlpmetrichttp.WithEndpoint(cmd.Flag(metricsEndpointFlag).Value.String()),
		}
		if ok, err := cmd.Flags().GetBool(metricsTlS); err != nil {
			panic(err)
		} else if !ok {
			opts = append(opts, otlpmetrichttp.WithInsecure())
		}

		ctx = WithNodeOptions(ctx, nodebuilder.WithMetrics(opts, NodeType(ctx)))
	}

	return ctx, err
}
