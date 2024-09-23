package p2p

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.uber.org/fx"

	"github.com/celestiaorg/celestia-node/nodebuilder/node"
)

// WithMetrics option sets up native libp2p metrics up.
func WithMetrics() fx.Option {
	return fx.Options(
		fx.Provide(resourceManagerOpt(traceReporter)),
		fx.Provide(prometheusMetrics),
		fx.Invoke(enableBitswapMetrics),
	)
}

const (
	promAgentEndpoint = "/metrics"
	promAgentPort     = "8890"

	networkLabel  = "network"
	nodeTypeLabel = "node_type"
	peerIDLabel   = "peer_id"
)

// prometheusMetrics option sets up native libp2p metrics up
func prometheusMetrics(lifecycle fx.Lifecycle,
	peerID peer.ID,
	nodeType node.Type,
	network Network,
) (prometheus.Registerer, error) {
	reg := prometheus.NewRegistry()
	labels := prometheus.Labels{
		networkLabel:  network.String(),
		nodeTypeLabel: nodeType.String(),
		peerIDLabel:   peerID.String(),
	}
	wrapped := prometheus.WrapRegistererWith(labels, reg)

	mux := http.NewServeMux()
	handler := promhttp.HandlerFor(reg, promhttp.HandlerOpts{Registry: wrapped})
	mux.Handle(promAgentEndpoint, handler)

	// TODO(@Wondertan): Unify all the servers into one (See #2007)
	promHTTPServer := &http.Server{
		Addr:              fmt.Sprintf(":%s", promAgentPort),
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
	}

	lifecycle.Append(fx.Hook{
		OnStart: func(_ context.Context) error {
			go func() {
				if err := promHTTPServer.ListenAndServe(); err != nil {
					log.Errorf("Error starting Prometheus metrics exporter http server: %s", err)
				}
			}()

			log.Infof("Prometheus agent started on :%s/%s", promAgentPort, promAgentEndpoint)
			return nil
		},
		OnStop: func(ctx context.Context) error {
			return promHTTPServer.Shutdown(ctx)
		},
	})
	return wrapped, nil
}
