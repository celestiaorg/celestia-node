package p2p

import (
	"os"
	"strconv"

	"github.com/libp2p/go-libp2p/core/peer"
	ma "github.com/multiformats/go-multiaddr"
)

const EnvKeyCelestiaBootstrapper = "CELESTIA_BOOTSTRAPPER"

func isBootstrapper() bool {
	return os.Getenv(EnvKeyCelestiaBootstrapper) == strconv.FormatBool(true)
}

// BootstrappersFor returns address information of bootstrap peers for a given network.
func BootstrappersFor(net Network) (Bootstrappers, error) {
	bs, err := bootstrappersFor(net)
	if err != nil {
		return nil, err
	}

	return parseAddrInfos(bs)
}

// bootstrappersFor reports multiaddresses of bootstrap peers for a given network.
func bootstrappersFor(net Network) ([]string, error) {
	var err error
	net, err = net.Validate()
	if err != nil {
		return nil, err
	}

	return bootstrapList[net], nil
}

// NOTE: Every time we add a new long-running network, its bootstrap peers have to be added here.
var bootstrapList = map[Network][]string{
	Arabica: {
		"/dns4/da-bridge-arabica-8.celestia-arabica.com/tcp/2121/p2p/12D3KooWDXkXARv79Dtn5xrGBgJePtCzCsEwWR7eGWnx9ZCyUyD6",
		"/dns4/da-bridge-arabica-8-2.celestia-arabica.com/tcp/2121/p2p/12D3KooWPu8qKmmNgYFMBsTkLBa1m3D9Cy9ReCAoQLqxEn9MHD1i",
		"/dns4/da-full-1-arabica-8.celestia-arabica.com/tcp/2121/p2p/12D3KooWEmeFodzypdTBTcw8Yub6WZRT4h1UgFtwCwwq6wS5Dtqm",
		"/dns4/da-full-2-arabica-8.celestia-arabica.com/tcp/2121/p2p/12D3KooWCs3wFmqwPn1u8pNU4BGsvLsob1ShTzvps8qEtTRuuuK5",
	},
	Mocha: {
		"/dns4/andromeda.celestia-devops.dev/tcp/2121/p2p/12D3KooWKvPXtV1yaQ6e3BRNUHa5Phh8daBwBi3KkGaSSkUPys6D",
		"/dns4/libra.celestia-devops.dev/tcp/2121/p2p/12D3KooWK5aDotDcLsabBmWDazehQLMsDkRyARm1k7f1zGAXqbt4",
		"/dns4/norma.celestia-devops.dev/tcp/2121/p2p/12D3KooWHYczJDVNfYVkLcNHPTDKCeiVvRhg8Q9JU3bE3m9eEVyY",
	},
	BlockspaceRace: {
		"/dns4/bootstr-incent-3.celestia.tools/tcp/2121/p2p/12D3KooWNzdKcHagtvvr6qtjcPTAdCN6ZBiBLH8FBHbihxqu4GZx",
		"/dns4/bootstr-incent-2.celestia.tools/tcp/2121/p2p/12D3KooWNJZyWeCsrKxKrxsNM1RVL2Edp77svvt7Cosa63TggC9m",
		"/dns4/bootstr-incent-1.celestia.tools/tcp/2121/p2p/12D3KooWBtxdBzToQwnS4ySGpph9PtGmmjEyATkgX3PfhAo4xmf7",
	},
	Private: {},
}

// parseAddrInfos converts strings to AddrInfos
func parseAddrInfos(addrs []string) ([]peer.AddrInfo, error) {
	infos := make([]peer.AddrInfo, 0, len(addrs))
	for _, addr := range addrs {
		maddr, err := ma.NewMultiaddr(addr)
		if err != nil {
			log.Errorw("parsing and validating addr", "addr", addr, "err", err)
			return nil, err
		}

		info, err := peer.AddrInfoFromP2pAddr(maddr)
		if err != nil {
			log.Errorw("parsing info from multiaddr", "maddr", maddr, "err", err)
			return nil, err
		}
		infos = append(infos, *info)
	}

	return infos, nil
}
