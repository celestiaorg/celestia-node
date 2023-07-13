package nodebuilder

import (
	"context"
	"fmt"
	"time"

	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/pyroscope-io/client/pyroscope"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	sdk "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	semconv "go.opentelemetry.io/otel/semconv/v1.11.0"
	"go.uber.org/fx"

	"github.com/celestiaorg/go-fraud"

	"github.com/celestiaorg/celestia-node/nodebuilder/das"
	modheader "github.com/celestiaorg/celestia-node/nodebuilder/header"
	"github.com/celestiaorg/celestia-node/nodebuilder/node"
	"github.com/celestiaorg/celestia-node/nodebuilder/p2p"
	"github.com/celestiaorg/celestia-node/nodebuilder/share"
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

// WithPyroscope enables pyroscope profiling for the node.
func WithPyroscope(endpoint string, nodeType node.Type) fx.Option {
	return fx.Options(
		fx.Invoke(func(peerID peer.ID) error {
			_, err := pyroscope.Start(pyroscope.Config{
				ApplicationName: "celestia.da-node",
				ServerAddress:   endpoint,
				Tags: map[string]string{
					"type":   nodeType.String(),
					"peerId": peerID.String(),
				},
				Logger: nil,
				ProfileTypes: []pyroscope.ProfileType{
					pyroscope.ProfileCPU,
					pyroscope.ProfileAllocObjects,
					pyroscope.ProfileAllocSpace,
					pyroscope.ProfileInuseObjects,
					pyroscope.ProfileInuseSpace,
				},
			})
			return err
		}),
	)
}

// WithMetrics enables metrics exporting for the node.
func WithMetrics(metricOpts []otlpmetrichttp.Option, nodeType node.Type, buildInfo node.BuildInfo) fx.Option {
	baseComponents := fx.Options(
		fx.Supply(metricOpts),
		fx.Supply(buildInfo),
		fx.Invoke(initializeMetrics),
		fx.Invoke(state.WithMetrics),
		fx.Invoke(fraud.WithMetrics),
		fx.Invoke(node.WithMetrics),
		fx.Invoke(modheader.WithMetrics),
		fx.Invoke(share.WithDiscoveryMetrics),
	)

	samplingMetrics := fx.Options(
		fx.Invoke(das.WithMetrics),
		fx.Invoke(share.WithPeerManagerMetrics),
		fx.Invoke(share.WithShrexClientMetrics),
		fx.Invoke(share.WithShrexGetterMetrics),
	)

	var opts fx.Option
	switch nodeType {
	case node.Full:
		opts = fx.Options(
			baseComponents,
			fx.Invoke(share.WithShrexServerMetrics),
			samplingMetrics,
		)
	case node.Light:
		opts = fx.Options(
			baseComponents,
			samplingMetrics,
		)
	case node.Bridge:
		opts = fx.Options(
			baseComponents,
			fx.Invoke(share.WithShrexServerMetrics),
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
	buildInfo node.BuildInfo,
	opts []otlpmetrichttp.Option,
) error {
	exp, err := otlpmetrichttp.New(ctx, opts...)
	if err != nil {
		return err
	}

	provider := sdk.NewMeterProvider(
		sdk.WithReader(sdk.NewPeriodicReader(exp, sdk.WithTimeout(2*time.Second))),
		sdk.WithResource(resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceNamespaceKey.String(fmt.Sprintf("Celestia-%s", nodeType.String())),
			semconv.ServiceNameKey.String(fmt.Sprintf("semver-%s", buildInfo.SemanticVersion)),
			semconv.ServiceInstanceIDKey.String(peerID.String()),
		)))
	lc.Append(fx.Hook{
		OnStop: func(ctx context.Context) error {
			return provider.Shutdown(ctx)
		},
	})
	otel.SetMeterProvider(provider)
	return nil
}
