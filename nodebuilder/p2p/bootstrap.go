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
		"/dns4/da-bridge-arabica-9.celestia-arabica.com/tcp/2121/p2p/12D3KooWBLvsfkbovAH74DbGGxHPpVW7DkvKdbQxhorrkv9tfGZU",
		"/dns4/da-bridge-arabica-9-2.celestia-arabica.com/tcp/2121/p2p/12D3KooWNjJSk8JcY7VoLEjGGUz8CXp9Bxt495zXmdmccjaMPgHf",
		"/dns4/da-full-1-arabica-9.celestia-arabica.com/tcp/2121/p2p/12D3KooWFUK2Z4WPsQN3p5n8tgBigxP32gbmABUet2UMby2Ha9ZK",
		"/dns4/da-full-2-arabica-9.celestia-arabica.com/tcp/2121/p2p/12D3KooWKnmwsimoghxUT1DXr7f8yXbWCfmXDk4UGbQDsAks9XsN",
	},
	Mocha: {
		"/dns4/bootstr-mocha-1.celestia-mocha.com/tcp/2121/p2p/12D3KooWDRSJMbH3PS4dRDa11H7Tk615aqTUgkeEKz4pwd4sS6fN",
		"/dns4/bootstr-mocha-2.celestia-mocha.com/tcp/2121/p2p/12D3KooWEk7cxtjQCC7kC84Uhs2j6dAHjdbwYnPcvUAqmj6Zsry2",
		"/dns4/bootstr-mocha-3.celestia-mocha.com/tcp/2121/p2p/12D3KooWBE4QcFXZzENf2VRo6Y5LBvp9gzmpYRHKCvgGzEYj7Hdn",
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
