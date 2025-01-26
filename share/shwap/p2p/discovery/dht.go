package discovery

import (
	"context"
	"fmt"
	"time"

	"github.com/ipfs/go-datastore"
	dht "github.com/libp2p/go-libp2p-kad-dht"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/protocol"
	"github.com/prometheus/client_golang/prometheus"
)

const (
	defaultRoutingRefreshPeriod = time.Minute
)

// PeerRouting provides a constructor for PeerRouting over DHT.
// Essentially, this offers a way to discover peer addresses by respecting public keys.
func NewDHT(
	ctx context.Context,
	prefix string,
	bootstrappers []peer.AddrInfo,
	host host.Host,
	dataStore datastore.Batching,
	mode dht.ModeOpt,
) (*dht.IpfsDHT, error) {
	opts := []dht.Option{
		dht.BootstrapPeers(bootstrappers...),
		dht.ProtocolPrefix(protocol.ID(fmt.Sprintf("/celestia/%s", prefix))),
		dht.Datastore(dataStore),
		dht.RoutingTableRefreshPeriod(defaultRoutingRefreshPeriod),
		dht.Mode(mode),
	}

	d, err := dht.New(ctx, host, opts...)
	if err != nil {
		return nil, err
	}

	// Create a metrics wrapper
	metricsDHT := NewMetricsDHT(d, prefix, mode)

	// Add event handlers for metrics
	metricsDHT.PeerConnected = func(id peer.ID) {
		dhtMetrics.PeersTotal.WithLabelValues(prefix, mode.String()).Inc()
		if metricsDHT.Host().Network().Connectedness(id) == network.DirInbound {
			dhtMetrics.InboundConnectionsTotal.WithLabelValues(prefix, mode.String()).Inc()
		} else {
			dhtMetrics.OutboundConnectionsTotal.WithLabelValues(prefix, mode.String()).Inc()
		}
	}

	metricsDHT.PeerDisconnected = func(id peer.ID) {
		dhtMetrics.PeersTotal.WithLabelValues(prefix, mode.String()).Dec()
	}

	// Add metrics for the routing table
	go func() {
		ticker := time.NewTicker(time.Minute)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				size := len(metricsDHT.RoutingTable().ListPeers())
				dhtMetrics.RoutingTableSize.WithLabelValues(prefix, mode.String()).Set(float64(size))
			}
		}
	}()

	// Add metrics for storage operations
	metricsDHT.Validator.RecordSize = func(key string, size int) {
		dhtMetrics.StoredRecordsTotal.WithLabelValues(prefix, mode.String()).Set(float64(size))
	}

	return metricsDHT.IpfsDHT, nil
}

// Add wrapper functions for tracking metrics

func trackDHTRequest(ctx context.Context, prefix, mode, requestType string) (context.Context, func(error)) {
	start := time.Now()
	return ctx, func(err error) {
		duration := time.Since(start)
		status := "success"
		if err != nil {
			status = "error"
		}
		
		dhtMetrics.RequestsTotal.WithLabelValues(prefix, mode, requestType, status).Inc()
		dhtMetrics.RequestDuration.WithLabelValues(prefix, mode, requestType).Observe(duration.Seconds())
	}
}

func trackFindPeer(ctx context.Context, prefix, mode string) func(error) {
	return func(err error) {
		status := "success"
		if err != nil {
			status = "error"
		}
		dhtMetrics.FindPeerTotal.WithLabelValues(prefix, mode, status).Inc()
	}
}

func trackFindProviders(ctx context.Context, prefix, mode string) func(error) {
	return func(err error) {
		status := "success"
		if err != nil {
			status = "error"
		}
		dhtMetrics.FindProvidersTotal.WithLabelValues(prefix, mode, status).Inc()
	}
}

func trackStoreOperation(ctx context.Context, prefix, mode string) func(error) {
	return func(err error) {
		status := "success"
		if err != nil {
			status = "error"
		}
		dhtMetrics.StoreOperationsTotal.WithLabelValues(prefix, mode, status).Inc()
	}
}

func trackRoutingTableRefresh(prefix, mode string) {
	dhtMetrics.RoutingTableRefreshes.WithLabelValues(prefix, mode).Inc()
}
