package getters

import "github.com/celestiaorg/celestia-node/share/p2p/peers"

type Option func(*ShrexGetter)

func WithArchivalPeerManager(manager *peers.Manager) func(*ShrexGetter) {
	return func(s *ShrexGetter) {
		s.archivalPeerManager = manager
	}
}
