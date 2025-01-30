package p2p

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	// DHTMetrics contains all metrics for the DHT server
	dhtMetrics = struct {
		// General DHT metrics
		PeersTotal *prometheus.GaugeVec
		RequestsTotal *prometheus.CounterVec
		RequestDuration *prometheus.HistogramVec

		// Metrics for search operations
		FindPeerTotal *prometheus.CounterVec
		FindProvidersTotal *prometheus.CounterVec

		// Metrics for storage operations
		StoredRecordsTotal *prometheus.GaugeVec
		StoreOperationsTotal *prometheus.CounterVec

		// Metrics for network operations
		InboundConnectionsTotal *prometheus.CounterVec
		OutboundConnectionsTotal *prometheus.CounterVec

		// Metrics for the routing table
		RoutingTableSize *prometheus.GaugeVec
		RoutingTableRefreshes *prometheus.CounterVec
	}{
		PeersTotal: promauto.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace: "celestia",
				Subsystem: "dht",
				Name:      "peers_total",
				Help:      "The total number of peers in the DHT",
			},
			[]string{networkLabel, nodeTypeLabel},
		),
		RequestsTotal: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: "celestia",
				Subsystem: "dht",
				Name:      "requests_total",
				Help:      "The total number of DHT requests",
			},
			[]string{networkLabel, nodeTypeLabel, "type", "status"},
		),
		RequestDuration: promauto.NewHistogramVec(
			prometheus.HistogramOpts{
				Namespace: "celestia",
				Subsystem: "dht",
				Name:      "request_duration_seconds",
				Help:      "Duration of DHT requests",
				Buckets:   prometheus.DefBuckets,
			},
			[]string{networkLabel, nodeTypeLabel, "type"},
		),
		FindPeerTotal: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: "celestia",
				Subsystem: "dht",
				Name:      "find_peer_total",
				Help:      "The total number of peer search operations",
			},
			[]string{networkLabel, nodeTypeLabel, "status"},
		),
		FindProvidersTotal: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: "celestia",
				Subsystem: "dht",
				Name:      "find_providers_total",
				Help:      "The total number of provider search operations",
			},
			[]string{networkLabel, nodeTypeLabel, "status"},
		),
		StoredRecordsTotal: promauto.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace: "celestia",
				Subsystem: "dht",
				Name:      "stored_records_total",
				Help:      "The total number of stored records",
			},
			[]string{networkLabel, nodeTypeLabel},
		),
		StoreOperationsTotal: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: "celestia",
				Subsystem: "dht",
				Name:      "store_operations_total",
				Help:      "The total number of storage operations",
			},
			[]string{networkLabel, nodeTypeLabel, "status"},
		),
		InboundConnectionsTotal: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: "celestia",
				Subsystem: "dht",
				Name:      "inbound_connections_total",
				Help:      "The total number of inbound connections",
			},
			[]string{networkLabel, nodeTypeLabel},
		),
		OutboundConnectionsTotal: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: "celestia",
				Subsystem: "dht",
				Name:      "outbound_connections_total",
				Help:      "The total number of outbound connections",
			},
			[]string{networkLabel, nodeTypeLabel},
		),
		RoutingTableSize: promauto.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace: "celestia",
				Subsystem: "dht",
				Name:      "routing_table_size",
				Help:      "The size of the routing table",
			},
			[]string{networkLabel, nodeTypeLabel},
		),
		RoutingTableRefreshes: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: "celestia",
				Subsystem: "dht",
				Name:      "routing_table_refreshes_total",
				Help:      "The total number of routing table refreshes",
			},
			[]string{networkLabel, nodeTypeLabel},
		),
	}
)
