package nodebuilder

import (
	"context"
	"fmt"
	"time"

	"github.com/libp2p/go-libp2p/core/peer"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	"go.opentelemetry.io/otel/metric/global"
	"go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	semconv "go.opentelemetry.io/otel/semconv/v1.11.0"
	"go.uber.org/fx"

	fraudPkg "github.com/celestiaorg/celestia-node/fraud"
	headerPkg "github.com/celestiaorg/celestia-node/header"

	"github.com/celestiaorg/celestia-node/nodebuilder/das"
	"github.com/celestiaorg/celestia-node/nodebuilder/node"
	"github.com/celestiaorg/celestia-node/nodebuilder/p2p"
	sharePkg "github.com/celestiaorg/celestia-node/nodebuilder/share"
	"github.com/celestiaorg/celestia-node/state"
)

// WithNetwork specifies the Network to which the Node should connect to.
// WARNING: Use this option with caution and never run the Node with different networks over the
// same persisted Store.
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
		fx.Invoke(headerPkg.WithMetrics),
		fx.Invoke(state.WithMetrics),
		fx.Invoke(p2p.WithMetrics),
		fx.Invoke(fraudPkg.WithMetrics),
		fx.Invoke(node.WithMetrics),
		// fx.Invoke(header.WithMetrics),
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

// WithBlackboxMetrics enables blackbox metrics for the node.
// Blackbox metrics are metrics that are recorded for the node's components
// through a proxy that records metrics for the node's components
// on each method call.
func WithBlackboxMetrics() fx.Option {
	return fx.Options(
		fx.Decorate(sharePkg.WithBlackBoxMetrics),
	)
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

	provider := metric.NewMeterProvider(
		metric.WithReader(metric.NewPeriodicReader(exp, metric.WithTimeout(2*time.Second))),
		metric.WithResource(resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceNameKey.String(fmt.Sprintf("Celestia-%s", nodeType.String())),
			// TODO(@Wondertan): Versioning: semconv.ServiceVersionKey
			semconv.ServiceInstanceIDKey.String(peerID.String()),
		)))

	lc.Append(fx.Hook{
		OnStop: func(ctx context.Context) error {
			return provider.Shutdown(ctx)
		},
	})
	global.SetMeterProvider(provider)

	return nil
}
