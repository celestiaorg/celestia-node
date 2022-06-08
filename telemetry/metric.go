package telemetry

import (
	"context"
	"log"
	"os"
	"time"

	"go.opentelemetry.io/otel/exporters/stdout/stdoutmetric"
	"go.opentelemetry.io/otel/metric/global"
	"go.opentelemetry.io/otel/metric/instrument/syncint64"
	controller "go.opentelemetry.io/otel/sdk/metric/controller/basic"
	processor "go.opentelemetry.io/otel/sdk/metric/processor/basic"
	"go.opentelemetry.io/otel/sdk/metric/selector/simple"
	"go.uber.org/fx"
)

var (
	sharesAvailableCounter syncint64.Counter
	sharesAvailableHist    syncint64.Histogram

	pusher      *controller.Controller
	metricsFile *os.File
)

func IncSharesAvailable(ctx context.Context) {
	sharesAvailableCounter.Add(ctx, 1)
}

func RecordSharesAvailable(ctx context.Context, dur time.Duration) {
	sharesAvailableHist.Record(ctx, dur.Microseconds())
}

func metrics() fx.Option {
	return fx.Invoke(
		func(lf fx.Lifecycle) {
			lf.Append(fx.Hook{
				OnStart: func(ctx context.Context) (err error) {
					metricsFile, err = os.Create("metrics.txt")
					if err != nil {
						return err
					}
					exporter, err := stdoutmetric.New(stdoutmetric.WithWriter(metricsFile), stdoutmetric.WithPrettyPrint())
					if err != nil {
						return err
					}

					pusher = controller.New(
						processor.NewFactory(
							simple.NewWithInexpensiveDistribution(),
							exporter,
						),
						controller.WithExporter(exporter),
					)
					if err = pusher.Start(ctx); err != nil {
						log.Fatalf("starting push controller: %v", err)
					}

					global.SetMeterProvider(pusher)
					meter := global.Meter(instrumentationName)

					sharesAvailableHist, err = meter.SyncInt64().Histogram("shares_available.duration")
					if err != nil {
						return err
					}
					sharesAvailableCounter, err = meter.SyncInt64().Counter("shares_available.total")
					if err != nil {
						return err
					}
					return nil
				},
				OnStop: func(ctx context.Context) error {
					defer metricsFile.Close()
					return pusher.Stop(ctx)
				},
			})
		},
	)
}
