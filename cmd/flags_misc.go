package cmd

import (
	"context"
	"fmt"
	"net/http"
	"net/http/pprof"
	"strings"

	otelpyroscope "github.com/grafana/otel-profiling-go"
	logging "github.com/ipfs/go-log/v2"
	"github.com/spf13/cobra"
	flag "github.com/spf13/pflag"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"

	"github.com/celestiaorg/celestia-node/logs"
	"github.com/celestiaorg/celestia-node/nodebuilder"
	modp2p "github.com/celestiaorg/celestia-node/nodebuilder/p2p"
)

var (
	LogLevelFlag        = "log.level"
	LogLevelModuleFlag  = "log.level.module"
	pprofFlag           = "pprof"
	tracingFlag         = "tracing"
	tracingEndpointFlag = "tracing.endpoint"
	tracingTlS          = "tracing.tls"
	metricsFlag         = "metrics"
	metricsEndpointFlag = "metrics.endpoint"
	metricsTlS          = "metrics.tls"
	p2pMetrics          = "p2p.metrics"
	pyroscopeFlag       = "pyroscope"
	pyroscopeTracing    = "pyroscope.tracing"
	pyroscopeEndpoint   = "pyroscope.endpoint"
)

// MiscFlags gives a set of hardcoded miscellaneous flags.
func MiscFlags() *flag.FlagSet {
	flags := &flag.FlagSet{}

	flags.String(
		LogLevelFlag,
		"INFO",
		`DEBUG, INFO, WARN, ERROR, DPANIC, PANIC, FATAL
and their lower-case forms`,
	)

	flags.StringSlice(
		LogLevelModuleFlag,
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

	flags.Bool(
		p2pMetrics,
		false,
		"Enable libp2p metrics",
	)

	flags.Bool(
		pyroscopeFlag,
		false,
		"Enables Pyroscope profiling",
	)

	flags.Bool(
		pyroscopeTracing,
		false,
		"Enables Pyroscope tracing integration. Depends on --tracing",
	)

	flags.String(
		pyroscopeEndpoint,
		"http://localhost:4040",
		"Sets HTTP endpoint for Pyroscope profiles to be exported to. Depends on '--pyroscope'",
	)

	return flags
}

// ParseMiscFlags parses miscellaneous flags from the given cmd and applies values to Env.
func ParseMiscFlags(ctx context.Context, cmd *cobra.Command) (context.Context, error) {
	logLevel := cmd.Flag(LogLevelFlag).Value.String()
	if logLevel != "" {
		level, err := logging.LevelFromString(logLevel)
		if err != nil {
			return ctx, fmt.Errorf("cmd: while parsing '%s': %w", LogLevelFlag, err)
		}

		logs.SetAllLoggers(level)
	}

	logModules, err := cmd.Flags().GetStringSlice(LogLevelModuleFlag)
	if err != nil {
		panic(err)
	}
	for _, ll := range logModules {
		params := strings.Split(ll, ":")
		if len(params) != 2 {
			return ctx, fmt.Errorf("cmd: %s arg must be in form <module>:<level>, e.g. pubsub:debug", LogLevelModuleFlag)
		}

		err := logging.SetLogLevel(params[0], params[1])
		if err != nil {
			return ctx, err
		}
	}

	enablePprof, err := cmd.Flags().GetBool(pprofFlag)
	if err != nil {
		panic(err)
	}

	if enablePprof {
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
			err := http.ListenAndServe("0.0.0.0:6000", mux) //nolint:gosec
			if err != nil {
				log.Fatalw("failed to start pprof server", "err", err)
			} else {
				log.Info("started pprof server on port 6000")
			}
		}()
	}

	enablePyro, err := cmd.Flags().GetBool(pyroscopeFlag)
	if err != nil {
		panic(err)
	}

	if enablePyro {
		ctx = WithNodeOptions(ctx,
			nodebuilder.WithPyroscope(
				cmd.Flag(pyroscopeEndpoint).Value.String(),
				NodeType(ctx),
			),
		)
	}

	enableTracing, err := cmd.Flags().GetBool(tracingFlag)
	if err != nil {
		panic(err)
	}

	if enableTracing {
		opts := []otlptracehttp.Option{
			otlptracehttp.WithCompression(otlptracehttp.GzipCompression),
			otlptracehttp.WithEndpoint(cmd.Flag(tracingEndpointFlag).Value.String()),
		}
		if ok, err := cmd.Flags().GetBool(tracingTlS); err != nil {
			panic(err)
		} else if !ok {
			opts = append(opts, otlptracehttp.WithInsecure())
		}

		pyroOpts := make([]otelpyroscope.Option, 0)
		enablePyroTracing, err := cmd.Flags().GetBool(pyroscopeTracing)
		if err != nil {
			panic(err)
		}
		if enablePyroTracing {
			pyroOpts = append(pyroOpts,
				otelpyroscope.WithAppName("celestia.da-node"),
				otelpyroscope.WithPyroscopeURL(cmd.Flag(pyroscopeEndpoint).Value.String()),
				otelpyroscope.WithRootSpanOnly(true),
				otelpyroscope.WithAddSpanName(true),
				otelpyroscope.WithProfileURL(true),
				otelpyroscope.WithProfileBaselineURL(true),
			)
		}
		ctx = WithNodeOptions(ctx, nodebuilder.WithTraces(opts, pyroOpts))
	}

	enableMetrics, err := cmd.Flags().GetBool(metricsFlag)
	if err != nil {
		panic(err)
	}

	if enableMetrics {
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

	enablep2pMetrics, err := cmd.Flags().GetBool(p2pMetrics)
	if err != nil {
		panic(err)
	}

	if enablep2pMetrics {
		if !enableMetrics {
			log.Error("--p2p.metrics used without --metrics being enabled")
		} else {
			ctx = WithNodeOptions(ctx, modp2p.WithMetrics())
		}
	}

	return ctx, err
}
