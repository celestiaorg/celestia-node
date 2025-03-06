package discovery

import (
	"context"
	"fmt"

	"github.com/ipfs/go-datastore"
	dht "github.com/libp2p/go-libp2p-kad-dht"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/protocol"
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
		dht.ProtocolPrefix(protocol.ID(fmt.Sprintf("/celestia/%s", prefix))),
		dht.Datastore(dataStore),
		dht.Mode(mode),
	}

	return dht.New(ctx, host, opts...)
}
