package discovery

import (
	"context"
	"fmt"
	"time"

	"github.com/ipfs/go-datastore"
	dht "github.com/libp2p/go-libp2p-kad-dht"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/protocol"
	"github.com/prometheus/client_golang/prometheus"
)

const (
	defaultRoutingRefreshPeriod = time.Minute
)

// PeerRouting provides constructor for PeerRouting over DHT.
// Basically, this provides a way to discover peer addresses by respecting public keys.
func NewDHT(
	ctx context.Context,
	prefix string,
	bootsrappers []peer.AddrInfo,
	host host.Host,
	dataStore datastore.Batching,
	mode dht.ModeOpt,
) (*dht.IpfsDHT, error) {
	// Create metrics registry with our labels
	reg := prometheus.NewRegistry()
	labels := prometheus.Labels{
		"network":   prefix,
		"node_type": mode.String(),
	}
	wrappedReg := prometheus.WrapRegistererWith(labels, reg)

	opts := []dht.Option{
		dht.BootstrapPeers(bootsrappers...),
		dht.ProtocolPrefix(protocol.ID(fmt.Sprintf("/celestia/%s", prefix))),
		dht.Datastore(dataStore),
		dht.RoutingTableRefreshPeriod(defaultRoutingRefreshPeriod),
		dht.Mode(mode),
		dht.Validator(dht.DefaultValidator{}),
		// Enable built-in DHT metrics
		dht.EnabledMetrics(wrappedReg),
	}

	return dht.New(ctx, host, opts...)
}
я
