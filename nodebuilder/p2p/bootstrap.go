package p2p

import (
	"context"
	"os"
	"strconv"
	"sync"

	"github.com/libp2p/go-libp2p/core/host"
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
	Mainnet: {
		"/dnsaddr/da-bridge-1.celestia-bootstrap.net/p2p/12D3KooWSqZaLcn5Guypo2mrHr297YPJnV8KMEMXNjs3qAS8msw8",
		"/dnsaddr/da-bridge-2.celestia-bootstrap.net/p2p/12D3KooWQpuTFELgsUypqp9N4a1rKBccmrmQVY8Em9yhqppTJcXf",
		"/dnsaddr/da-full-1.celestia-bootstrap.net/p2p/12D3KooWKZCMcwGCYbL18iuw3YVpAZoyb1VBGbx9Kapsjw3soZgr",
		"/dnsaddr/da-full-2.celestia-bootstrap.net/p2p/12D3KooWE3fmRtHgfk9DCuQFfY3H3JYEnTU3xZozv1Xmo8KWrWbK",
		"/dnsaddr/boot.celestia.pops.one/p2p/12D3KooWBBzzGy5hAHUQVh2vBvL25CKwJ7wbmooPcz4amQhzJHJq",
		"/dnsaddr/celestia.qubelabs.io/p2p/12D3KooWAzucnC7yawvLbmVxv53ihhjbHFSVZCsPuuSzTg6A7wgx",
	},
	Arabica: {
		"/dnsaddr/da-bridge-1.celestia-arabica-11.com/p2p/12D3KooWGqwzdEqM54Dce6LXzfFr97Bnhvm6rN7KM7MFwdomfm4S",
		"/dnsaddr/da-full-1.celestia-arabica-11.com/p2p/12D3KooWCMGM5eZWVfCN9ZLAViGfLUWAfXP5pCm78NFKb9jpBtua",
	},
	Mocha: {
		"/dnsaddr/da-bridge-1-mocha-4.celestia-mocha.com/p2p/12D3KooWCBAbQbJSpCpCGKzqz3rAN4ixYbc63K68zJg9aisuAajg",
		"/dnsaddr/da-full-1-mocha-4.celestia-mocha.com/p2p/12D3KooWCUHPLqQXZzpTx1x3TAsdn3vYmTNDhzg66yG8hqoxGGN8",
		"/dnsaddr/mocha-boot.pops.one/p2p/12D3KooWDzNyDSvTBdKQAmnsUdAyQCQWwM3ReXTmPaaf6LzfNwRs",
		"/dnsaddr/celestia-mocha.qubelabs.io/p2p/12D3KooWQVmHy7JpfxpKZfLjvn12GjvMgKrWdsHkFbV2kKqQFBCG",
		"/dnsaddr/celestia-mocha4-bootstrapper.binary.builders/p2p/12D3KooWK6AYaPSe2EP99NP5G2DKwWLfMi6zHMYdD65KRJwdJSVU",
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

// connectToBootstrappers ensures that the bootstrapper node
// initiates a connection to other hardcoded bootstrap peers
// while the connectionManager hook adds them as mutual peers to prevent
// trimming the connection. This will aid the network's connectivity.
func connectToBootstrappers(ctx context.Context, h host.Host, network Network) error {
	if !isBootstrapper() {
		return nil
	}

	boots, err := BootstrappersFor(network)
	if err != nil {
		return err
	}

	wg := sync.WaitGroup{}
	wg.Add(len(boots))
	for _, b := range boots {
		go func() {
			defer wg.Done()

			err = h.Connect(ctx, b)
			if err != nil {
				log.Errorw("bootstrap: failed to connect to bootstrapper", "id", b, "err", err)
				return
			}

			log.Infow("bootstrap: successfully connected to bootstrapper", "id", b.String())
		}()
	}
	wg.Wait()

	return nil
}
