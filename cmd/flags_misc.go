package cmd

import (
	"fmt"
	"log"
	"net/http"
	"net/http/pprof"
	"strings"

	logging "github.com/ipfs/go-log/v2"
	"github.com/spf13/cobra"
	flag "github.com/spf13/pflag"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/sdk/resource"
	tracesdk "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.10.0"

	"github.com/celestiaorg/celestia-node/logs"
)

var (
	logLevelFlag        = "log.level"
	logLevelModuleFlag  = "log.level.module"
	pprofFlag           = "pprof"
	tracingFlag         = "tracing"
	tracingEndpointFlag = "tracing.endpoint"
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

	return flags
}

// ParseMiscFlags parses miscellaneous flags from the given cmd and applies values to Env.
func ParseMiscFlags(cmd *cobra.Command, env *Env) error {
	logLevel := cmd.Flag(logLevelFlag).Value.String()
	if logLevel != "" {
		level, err := logging.LevelFromString(logLevel)
		if err != nil {
			return fmt.Errorf("cmd: while parsing '%s': %w", logLevelFlag, err)
		}

		logs.SetAllLoggers(level)
	}

	logModules, err := cmd.Flags().GetStringSlice(logLevelModuleFlag)
	if err != nil {
		return err
	}
	for _, ll := range logModules {
		params := strings.Split(ll, ":")
		if len(params) != 2 {
			return fmt.Errorf("cmd: %s arg must be in form <module>:<level>, e.g. pubsub:debug", logLevelModuleFlag)
		}

		err := logging.SetLogLevel(params[0], params[1])
		if err != nil {
			return err
		}
	}

	ok, err := cmd.Flags().GetBool(pprofFlag)
	if err != nil {
		return err
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
			log.Println(http.ListenAndServe("0.0.0.0:6000", mux))
		}()
	}

	ok, err = cmd.Flags().GetBool(tracingFlag)
	if ok {
		exp, err := otlptracehttp.New(cmd.Context(),
			otlptracehttp.WithEndpoint(cmd.Flag(tracingEndpointFlag).Value.String()),
			otlptracehttp.WithCompression(otlptracehttp.GzipCompression),
			otlptracehttp.WithInsecure(),
		)
		if err != nil {
			return err
		}

		tp := tracesdk.NewTracerProvider(
			// Always be sure to batch in production.
			tracesdk.WithBatcher(exp),
			// Record information about this application in a Resource.
			tracesdk.WithResource(resource.NewWithAttributes(
				semconv.SchemaURL,
				semconv.ServiceNameKey.String(fmt.Sprintf("Celestia-%s", env.NodeType.String())),
				// TODO(@Wondertan): Versioning: semconv.ServiceVersionKey
			)),
		)
		otel.SetTracerProvider(tp)
	}

	return err
}
