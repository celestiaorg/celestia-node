package p2p

import (
	"context"
	"fmt"
	"net/http"
	"time"

	share "github.com/celestiaorg/celestia-node/share"
	rcmgrObs "github.com/libp2p/go-libp2p/p2p/host/resource-manager/obs"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.uber.org/fx"
)

// WithMetrics option sets up native libp2p metrics up.
func WithMetrics() fx.Option {
	return fx.Options(
		fx.Provide(resourceManagerOpt(traceReporter)),
		fx.Provide(prometheusRegisterer),
		fx.Invoke(prometheusMetrics),
	)
}

const (
	promAgentEndpoint = "/metrics"
	promAgentPort     = "8890"
)

// option sets up native libp2p metrics up
func prometheusMetrics(lifecycle fx.Lifecycle, registerer prometheus.Registerer) error {

	// Register the semanticVersion as a metric
	semanticVersionMetric := prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "celestia_node_version",
			Help: "Semantic version of Celestia Node",
		},
		[]string{"version"},
	)

	// Register the metric with the registerer
	registerer.MustRegister(semanticVersionMetric)

	// Retrieve the semanticVersion value using the exported function from cmd/celestia
	semanticVersion := share.GetSemanticVersion()

	// Set the value of semanticVersionMetric with the version label
	semanticVersionMetric.WithLabelValues(semanticVersion).Set(1)

	// Retrieve the semanticVersion value using the exported function from cmd/celestia
	fmt.Println("-----------------------------------")
	fmt.Println("- VERSION -", semanticVersion)
	fmt.Println("-----------------------------------")
	rcmgrObs.MustRegisterWith(registerer)

	registry := registerer.(*prometheus.Registry)

	mux := http.NewServeMux()
	handler := promhttp.HandlerFor(registry, promhttp.HandlerOpts{Registry: registerer})
	mux.Handle(promAgentEndpoint, handler)

	// TODO(@Wondertan): Unify all the servers into one (See #2007)
	promHTTPServer := &http.Server{
		Addr:              fmt.Sprintf(":%s", promAgentPort),
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
	}

	lifecycle.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
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
	return nil
}

func prometheusRegisterer() prometheus.Registerer {
	return prometheus.NewRegistry()
}
