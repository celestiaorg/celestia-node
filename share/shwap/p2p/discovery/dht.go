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
)

const (
	defaultRoutingRefreshPeriod = time.Minute

	// discovery version is a prefix for all tags used in discovery. It is bumped when
	// there are protocol breaking changes.
	version = "v1"
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
	opts := []dht.Option{
		dht.BootstrapPeers(bootsrappers...),
		dht.ProtocolPrefix(protocol.ID(fmt.Sprintf("/celestia/%s/%s", prefix, version))),
		dht.Datastore(dataStore),
		dht.RoutingTableRefreshPeriod(defaultRoutingRefreshPeriod),
		dht.Mode(mode),
	}

	return dht.New(ctx, host, opts...)
}
