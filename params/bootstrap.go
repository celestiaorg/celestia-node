package params

import (
	"github.com/libp2p/go-libp2p-core/peer"
	ma "github.com/multiformats/go-multiaddr"
)

// BootstrappersInfos returns address information of bootstrap peers for the node's current network.
func BootstrappersInfos() []peer.AddrInfo {
	bs := Bootstrappers()
	maddrs := make([]ma.Multiaddr, len(bs))
	for i, addr := range bs {
		maddr, err := ma.NewMultiaddr(addr)
		if err != nil {
			panic(err)
		}
		maddrs[i] = maddr
	}

	infos, err := peer.AddrInfosFromP2pAddrs(maddrs...)
	if err != nil {
		panic(err)
	}

	return infos
}

// Bootstrappers reports multiaddresses of bootstrap peers for the node's current network.
func Bootstrappers() []string {
	return bootstrapList[network] // network is guaranteed to be valid
}

// BootstrappersFor reports multiaddresses of bootstrap peers for a given network.
func BootstrappersFor(net Network) ([]string, error) {
	if err := net.Validate(); err != nil {
		return nil, err
	}

	return bootstrapList[net], nil
}

// NOTE: Every time we add a new long-running network, its bootstrap peers have to be added here.
var bootstrapList = map[Network][]string{
	DevNet: {
		"/dns4/andromeda.celestia-devops.dev/tcp/2121/p2p/12D3KooWKvPXtV1yaQ6e3BRNUHa5Phh8daBwBi3KkGaSSkUPys6D",
		"/dns4/libra.celestia-devops.dev/tcp/2121/p2p/12D3KooWK5aDotDcLsabBmWDazehQLMsDkRyARm1k7f1zGAXqbt4",
		"/dns4/norma.celestia-devops.dev/tcp/2121/p2p/12D3KooWHYczJDVNfYVkLcNHPTDKCeiVvRhg8Q9JU3bE3m9eEVyY",
	},
}
