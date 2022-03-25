package params

import (
	"os"
	"strings"
	"sync"

	"github.com/libp2p/go-libp2p-core/peer"
	ma "github.com/multiformats/go-multiaddr"
)

// BootstrappersInfos returns address information of predefined bootstrap peers for the node's current network.
// Use CELESTIA_BOOTSTRAPPERS env var to set custom bootstrap peers.
func BootstrappersInfos() []peer.AddrInfo {
	infosLk.Lock()
	defer infosLk.Unlock()
	if infos != nil {
		return infos
	}

	var err error
	infos, err = parseAddrInfos(Bootstrappers())
	if err != nil {
		panic(err)
	}
	return infos
}

// Bootstrappers reports multiaddresses of predefined bootstrap peers for the node's current network.
// Use CELESTIA_BOOTSTRAPPERS env var to set custom bootstrap peers.
func Bootstrappers() []string {
	return bootstrapList[network] // network is guaranteed to be valid
}

// BootstrappersFor reports multiaddresses of predefined bootstrap peers for a given network.
// Use CELESTIA_BOOTSTRAPPERS env var to set custom bootstrap peers.
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
	Private: {},
}

func init() {
	if netbootstrappers, ok := os.LookupEnv("CELESTIA_BOOTSTRAPPERS"); ok {
		netbootstrappers := strings.Split(netbootstrappers, ":")
		if len(netbootstrappers) != 2 {
			panic("env CELESTIA_BOOTSTRAPPERS: must be formatted as 'network:multiaddr1,multiaddr2...'")
		}

		net, bootstrappers := Network(netbootstrappers[0]), netbootstrappers[1]
		if err := net.Validate(); err != nil {
			println("env CELESTIA_BOOTSTRAPPERS")
			panic(err)
		}
		bs := strings.Split(bootstrappers, ",")

		// validate correctness
		_, err := parseAddrInfos(bs)
		if err != nil {
			println("env CELESTIA_BOOTSTRAPPERS: contains invalid multiaddress")
			panic(err)
		}

		bootstrapList[net] = bs
	}
}

func parseAddrInfos(addrs []string) ([]peer.AddrInfo, error) {
	infos := make([]peer.AddrInfo, len(addrs))
	for _, addr := range addrs {
		maddr, err := ma.NewMultiaddr(addr)
		if err != nil {
			return nil, err
		}

		info, err := peer.AddrInfoFromP2pAddr(maddr)
		if err != nil {
			return nil, err
		}
		infos = append(infos, *info)
	}

	return infos, nil
}

var (
	infosLk sync.Mutex
	infos   []peer.AddrInfo
)
