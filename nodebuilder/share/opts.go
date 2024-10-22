package share

import (
	"errors"

	"github.com/celestiaorg/celestia-node/share/shwap/p2p/discovery"
	"github.com/celestiaorg/celestia-node/share/shwap/p2p/shrex/peers"
	"github.com/celestiaorg/celestia-node/share/shwap/p2p/shrex/shrex_getter"
	"github.com/celestiaorg/celestia-node/share/shwap/p2p/shrex/shrexeds"
	"github.com/celestiaorg/celestia-node/share/shwap/p2p/shrex/shrexnd"
	"github.com/celestiaorg/celestia-node/store"
)

// WithPeerManagerMetrics is a utility function to turn on peer manager metrics and that is
// expected to be "invoked" by the fx lifecycle.
func WithPeerManagerMetrics(managers map[string]*peers.Manager) error {
	var err error
	for _, m := range managers {
		err = errors.Join(err, m.WithMetrics())
	}
	return err
}

// WithDiscoveryMetrics is a utility function to turn on discovery metrics and that is expected to
// be "invoked" by the fx lifecycle.
func WithDiscoveryMetrics(discs []*discovery.Discovery) error {
	var err error
	for _, disc := range discs {
		err = errors.Join(err, disc.WithMetrics())
	}
	return err
}

func WithShrexClientMetrics(edsClient *shrexeds.Client, ndClient *shrexnd.Client) error {
	err := edsClient.WithMetrics()
	if err != nil {
		return err
	}

	return ndClient.WithMetrics()
}

func WithShrexServerMetrics(edsServer *shrexeds.Server, ndServer *shrexnd.Server) error {
	err := edsServer.WithMetrics()
	if err != nil {
		return err
	}

	return ndServer.WithMetrics()
}

func WithShrexGetterMetrics(sg *shrex_getter.Getter) error {
	return sg.WithMetrics()
}

func WithStoreMetrics(s *store.Store) error {
	return s.WithMetrics()
}
