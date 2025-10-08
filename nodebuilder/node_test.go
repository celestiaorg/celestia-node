//go:build !race

package nodebuilder

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	collectormetricpb "go.opentelemetry.io/proto/otlp/collector/metrics/v1"
	"google.golang.org/protobuf/proto"

	"github.com/celestiaorg/celestia-node/core"
	"github.com/celestiaorg/celestia-node/nodebuilder/node"
)

func TestLifecycle(t *testing.T) {
	test := []struct {
		tp node.Type
	}{
		{tp: node.Bridge},
		{tp: node.Full},
		{tp: node.Light},
	}

	for i, tt := range test {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			// we're also creating a test node because the gRPC connection
			// is started automatically when starting the node.
			consNode := core.StartTestNode(t)
			host, port, err := net.SplitHostPort(consNode.GRPCClient.Target())
			require.NoError(t, err)

			cfg := DefaultConfig(tt.tp)
			cfg.Core.IP = host
			cfg.Core.Port = port

			node := TestNodeWithConfig(t, tt.tp, cfg)
			require.NotNil(t, node)
			require.NotNil(t, node.Config)
			require.NotNil(t, node.Host)
			require.NotNil(t, node.HeaderServ)
			require.NotNil(t, node.StateServ)
			require.NotNil(t, node.AdminServ)
			require.Equal(t, tt.tp, node.Type)

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			err = node.Start(ctx)
			require.NoError(t, err)

			err = node.Stop(ctx)
			require.NoError(t, err)
		})
	}
}

func TestLifecycle_WithMetrics(t *testing.T) {
	url, stop := StartMockOtelCollectorHTTPServer(t)
	defer stop()

	otelCollectorURL := strings.ReplaceAll(url, "http://", "")

	test := []struct {
		tp           node.Type
		coreExpected bool
	}{
		{tp: node.Bridge},
		{tp: node.Full},
		{tp: node.Light},
	}

	consNode := core.StartTestNode(t)
	host, port, err := net.SplitHostPort(consNode.GRPCClient.Target())
	require.NoError(t, err)

	for i, tt := range test {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			// we're also creating a test node because the gRPC connection
			// is started automatically when starting the node.

			cfg := DefaultConfig(tt.tp)
			cfg.Core.IP = host
			cfg.Core.Port = port

			node := TestNodeWithConfig(
				t,
				tt.tp,
				cfg,
				WithMetrics(
					[]otlpmetrichttp.Option{
						otlpmetrichttp.WithEndpoint(otelCollectorURL),
						otlpmetrichttp.WithInsecure(),
					},
					tt.tp,
				),
			)
			require.NotNil(t, node)
			require.NotNil(t, node.Config)
			require.NotNil(t, node.Host)
			require.NotNil(t, node.HeaderServ)
			require.NotNil(t, node.StateServ)
			require.Equal(t, tt.tp, node.Type)

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			err = node.Start(ctx)
			require.NoError(t, err)

			err = node.Stop(ctx)
			require.NoError(t, err)
		})
	}
}

func StartMockOtelCollectorHTTPServer(t *testing.T) (string, func()) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/metrics" && r.Method != http.MethodPost {
			t.Errorf("Expected to request [POST] '/fixedvalue', got: [%s] %s", r.Method, r.URL.Path)
		}

		if r.Header.Get("Content-Type") != "application/x-protobuf" {
			t.Errorf("Expected Content-Type: application/x-protobuf header, got: %s", r.Header.Get("Content-Type"))
		}

		response := collectormetricpb.ExportMetricsServiceResponse{}
		rawResponse, _ := proto.Marshal(&response)
		contentType := "application/x-protobuf"
		status := http.StatusOK

		log.Debug("Responding to otlp POST request")
		w.Header().Set("Content-Type", contentType)
		w.WriteHeader(status)
		_, _ = w.Write(rawResponse)

		log.Debug("Responded to otlp POST request")
	}))

	server.EnableHTTP2 = true
	return server.URL, server.Close
}
