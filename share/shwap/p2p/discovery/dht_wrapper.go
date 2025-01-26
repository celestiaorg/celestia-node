package discovery

import (
	"context"
	"time"

	dht "github.com/libp2p/go-libp2p-kad-dht"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/routing"
)

// MetricsDHT wraps DHT for collecting metrics
type MetricsDHT struct {
	*dht.IpfsDHT
	prefix string
	mode   string
}

// NewMetricsDHT creates a new DHT wrapper with metrics
func NewMetricsDHT(d *dht.IpfsDHT, prefix string, mode dht.ModeOpt) *MetricsDHT {
	return &MetricsDHT{
		IpfsDHT: d,
		prefix:  prefix,
		mode:    mode.String(),
	}
}

// FindPeer wraps the original method for collecting metrics
func (m *MetricsDHT) FindPeer(ctx context.Context, id peer.ID) (peer.AddrInfo, error) {
	ctx, done := trackDHTRequest(ctx, m.prefix, m.mode, "find_peer")
	defer trackFindPeer(ctx, m.prefix, m.mode)(nil)
	
	info, err := m.IpfsDHT.FindPeer(ctx, id)
	done(err)
	return info, err
}

// Provide wraps the original method for collecting metrics
func (m *MetricsDHT) Provide(ctx context.Context, key routing.Cid, announce bool) error {
	ctx, done := trackDHTRequest(ctx, m.prefix, m.mode, "provide")
	defer trackStoreOperation(ctx, m.prefix, m.mode)(nil)
	
	err := m.IpfsDHT.Provide(ctx, key, announce)
	done(err)
	return err
}

// FindProvidersAsync wraps the original method for collecting metrics
func (m *MetricsDHT) FindProvidersAsync(ctx context.Context, key routing.Cid, count int) <-chan peer.AddrInfo {
	ctx, done := trackDHTRequest(ctx, m.prefix, m.mode, "find_providers")
	defer trackFindProviders(ctx, m.prefix, m.mode)(nil)
	
	ch := m.IpfsDHT.FindProvidersAsync(ctx, key, count)
	done(nil) // Here, we cannot track errors since the method is asynchronous
	return ch
}

// Bootstrap wraps the original method for collecting metrics
func (m *MetricsDHT) Bootstrap(ctx context.Context) error {
	ctx, done := trackDHTRequest(ctx, m.prefix, m.mode, "bootstrap")
	defer trackRoutingTableRefresh(m.prefix, m.mode)
	
	err := m.IpfsDHT.Bootstrap(ctx)
	done(err)
	return err
}

// PutValue wraps the original method for collecting metrics
func (m *MetricsDHT) PutValue(ctx context.Context, key string, value []byte, opts ...routing.Option) error {
	ctx, done := trackDHTRequest(ctx, m.prefix, m.mode, "put_value")
	defer trackStoreOperation(ctx, m.prefix, m.mode)(nil)
	
	err := m.IpfsDHT.PutValue(ctx, key, value, opts...)
	done(err)
	return err
}

// GetValue wraps the original method for collecting metrics
func (m *MetricsDHT) GetValue(ctx context.Context, key string, opts ...routing.Option) ([]byte, error) {
	ctx, done := trackDHTRequest(ctx, m.prefix, m.mode, "get_value")
	
	val, err := m.IpfsDHT.GetValue(ctx, key, opts...)
	done(err)
	return val, err
}
