package telemetry

import (
	"context"
	"time"

	"go.uber.org/fx"
)

type Module interface {
	// RecordNodeStartTime records the time when the node started.
	RecordNodeStartTime(ctx context.Context)

	// ObserveNodeUptime records the total time the node has been running.
	ObserveNodeUptime(ctx context.Context)
}

func ConstructModule(cfg *Config) fx.Option {
	baseComponents := fx.Options(
		fx.Supply(*cfg),
	)

	if cfg.Enabled {
		return fx.Module(
			"telemetry",
			baseComponents,
			fx.Provide(
				fx.Annotate(
					newNodeMetrics,
					fx.OnStart(func(startCtx, ctx context.Context, m *metrics, cfg *Config) error {
						m.RecordNodeStartTime(ctx)

						interval := time.Duration(cfg.NodeUptimeScrapeInterval) * time.Minute

						go m.ObserveNodeUptime(ctx, interval)
						return nil
					}),
				),
			),
		)
	}

	return fx.Option(nil)
}
