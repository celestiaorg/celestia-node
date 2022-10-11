package nodebuilder

import (
	"context"
	"fmt"
	"time"

	"github.com/libp2p/go-libp2p-core/peer"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	"go.opentelemetry.io/otel/metric/global"
	controller "go.opentelemetry.io/otel/sdk/metric/controller/basic"
	processor "go.opentelemetry.io/otel/sdk/metric/processor/basic"
	selector "go.opentelemetry.io/otel/sdk/metric/selector/simple"
	"go.opentelemetry.io/otel/sdk/resource"
	semconv "go.opentelemetry.io/otel/semconv/v1.10.0"
	"go.uber.org/fx"

	"github.com/celestiaorg/celestia-node/fraud"
	"github.com/celestiaorg/celestia-node/header"
	"github.com/celestiaorg/celestia-node/nodebuilder/das"
	"github.com/celestiaorg/celestia-node/nodebuilder/node"
	"github.com/celestiaorg/celestia-node/nodebuilder/p2p"
	"github.com/celestiaorg/celestia-node/state"
)

// WithNetwork specifies the Network to which the Node should connect to.
// WARNING: Use this option with caution and never run the Node with different networks over the same persisted Store.
func WithNetwork(net p2p.Network) fx.Option {
	return fx.Replace(net)
}

// WithBootstrappers sets custom bootstrap peers.
func WithBootstrappers(peers p2p.Bootstrappers) fx.Option {
	return fx.Replace(peers)
}

// WithMetrics enables metrics exporting for the node.
func WithMetrics(metricOpts []otlpmetrichttp.Option, nodeType node.Type) fx.Option {
	baseComponents := fx.Options(
		fx.Supply(metricOpts),
		fx.Invoke(initializeMetrics),
		fx.Invoke(header.WithMetrics),
		fx.Invoke(state.WithMetrics),
		fx.Invoke(fraud.WithMetrics),
	)

	var opts fx.Option
	switch nodeType {
	case node.Full, node.Light:
		opts = fx.Options(
			baseComponents,
			fx.Invoke(das.WithMetrics),
			// add more monitoring here
		)
	case node.Bridge:
		opts = fx.Options(
			baseComponents,
			// add more monitoring here
		)
	default:
		panic("invalid node type")
	}
	return opts
}

// initializeMetrics initializes the global meter provider.
func initializeMetrics(
	ctx context.Context,
	lc fx.Lifecycle,
	peerID peer.ID,
	nodeType node.Type,
	opts []otlpmetrichttp.Option,
) error {
	exp, err := otlpmetrichttp.New(ctx, opts...)
	if err != nil {
		return err
	}

	pusher := controller.New(
		processor.NewFactory(
			selector.NewWithHistogramDistribution(),
			exp,
		),
		controller.WithExporter(exp),
		controller.WithCollectPeriod(2*time.Second),
		controller.WithResource(resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceNameKey.String(fmt.Sprintf("Celestia-%s", nodeType.String())),
			// TODO(@Wondertan): Versioning: semconv.ServiceVersionKey
			semconv.ServiceInstanceIDKey.String(peerID.String()),
		)),
	)

	lc.Append(fx.Hook{
		// here we take the context from fx.Invoke because pusher uses it for its entire lifetime,
		// instead of only for the Start operation
		OnStart: func(context.Context) error {
			return pusher.Start(ctx)
		},
		OnStop: func(ctx context.Context) error {
			return pusher.Stop(ctx)
		},
	})

	global.SetMeterProvider(pusher)
	return nil
}
