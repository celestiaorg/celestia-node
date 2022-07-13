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
	"go.opentelemetry.io/otel/exporters/jaeger"
	"go.opentelemetry.io/otel/sdk/resource"
	tracesdk "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.10.0"

	"github.com/celestiaorg/celestia-node/logs"
)

var (
	logLevelFlag       = "log.level"
	logLevelModuleFlag = "log.level.module"
	tracingJaegerFlag  = "tracing.jaeger"
	pprofFlag          = "pprof"
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

	flags.String(
		tracingJaegerFlag,
		"",
		"Enables Jaeger tracing. Expects an URL",
	)

	flags.Bool(
		pprofFlag,
		false,
		"Enables standard profiling handler (pprof) and exposes the profiles on port 6000",
	)

	return flags
}

// ParseMiscFlags parses miscellaneous flags from the given cmd and applies values to Env.
func ParseMiscFlags(cmd *cobra.Command) error {
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

	tracingJaegerURL := cmd.Flag(tracingJaegerFlag).Value.String()
	if tracingJaegerURL != "" {
		exp, err := jaeger.New(jaeger.WithCollectorEndpoint(jaeger.WithEndpoint(tracingJaegerURL)))
		if err != nil {
			return err
		}

		tp := tracesdk.NewTracerProvider(
			// Always be sure to batch in production.
			tracesdk.WithBatcher(exp),
			// Record information about this application in a Resource.
			tracesdk.WithResource(resource.NewWithAttributes(
				semconv.SchemaURL,
				semconv.ServiceNameKey.String("celestia-node"),
			)),
		)
		otel.SetTracerProvider(tp)
	}

	ok, err := cmd.Flags().GetBool(pprofFlag)
	if ok {
		// TODO(@Wondertan): Eventually, this should be registered on http server in RPC
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
	return err
}
