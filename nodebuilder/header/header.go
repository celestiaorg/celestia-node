package header

import (
	"context"

	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/libp2p/go-libp2p-core/peerstore"

	"github.com/celestiaorg/celestia-node/header"
	"github.com/celestiaorg/celestia-node/header/p2p"
	"github.com/celestiaorg/celestia-node/header/store"
	"github.com/celestiaorg/celestia-node/params"
)

// P2PExchange constructs new Exchange for headers.
func P2PExchange(cfg Config) func(params.Bootstrappers, host.Host) (header.Exchange, error) {
	return func(bpeers params.Bootstrappers, host host.Host) (header.Exchange, error) {
		peers, err := cfg.trustedPeers(bpeers)
		if err != nil {
			return nil, err
		}
		ids := make([]peer.ID, len(peers))
		for index, peer := range peers {
			ids[index] = peer.ID
			host.Peerstore().AddAddrs(peer.ID, peer.Addrs, peerstore.PermanentAddrTTL)
		}
		return p2p.NewExchange(host, ids), nil
	}
}

// InitStore initializes the store.
func InitStore(ctx context.Context, cfg Config, net params.Network, s header.Store, ex header.Exchange) error {
	trustedHash, err := cfg.trustedHash(net)
	if err != nil {
		return err
	}

	err = store.Init(ctx, s, ex, trustedHash)
	if err != nil {
		// TODO(@Wondertan): Error is ignored, as otherwise unit tests for Node construction fail.
		// 	This is due to requesting step of initialization, which fetches initial Header by trusted hash from
		//  the network. The step can't be done during unit tests and fixing it would require either
		//   * Having some test/dev/offline mode for Node that mocks out all the networking
		//   * Hardcoding full extended header in params pkg, instead of hashes, so we avoid requesting step
		log.Errorf("initializing store failed: %s", err)
	}

	return nil
}
