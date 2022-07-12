package node

import (
	"encoding/hex"
	"time"

	"github.com/libp2p/go-libp2p-core/crypto"
	"github.com/libp2p/go-libp2p-core/host"
	"go.uber.org/fx"

	apptypes "github.com/celestiaorg/celestia-app/x/payment/types"
	"github.com/celestiaorg/celestia-node/core"
	"github.com/celestiaorg/celestia-node/header"
	"github.com/celestiaorg/celestia-node/libs/fxutil"
	"github.com/celestiaorg/celestia-node/node/p2p"
	"github.com/celestiaorg/celestia-node/params"
)

// settings store values that can be augmented or changed for Node with Options.
type settings struct {
	cfg  *Config
	opts []fx.Option
}

// Option for Node's Config.
type Option func(*settings)

// WithNetwork specifies the Network to which the Node should connect to.
// WARNING: Use this option with caution and never run the Node with different networks over the same persisted Store.
func WithNetwork(net params.Network) Option {
	return func(sets *settings) {
		sets.opts = append(sets.opts, fx.Replace(net))
	}
}

// WithP2PKey sets custom Ed25519 private key for p2p networking.
func WithP2PKey(key crypto.PrivKey) Option {
	return func(sets *settings) {
		sets.opts = append(sets.opts, fxutil.ReplaceAs(key, new(crypto.PrivKey)))
	}
}

// WithP2PKeyStr sets custom hex encoded Ed25519 private key for p2p networking.
func WithP2PKeyStr(key string) Option {
	return func(sets *settings) {
		decKey, err := hex.DecodeString(key)
		if err != nil {
			sets.opts = append(sets.opts, fx.Error(err))
			return
		}

		key, err := crypto.UnmarshalEd25519PrivateKey(decKey)
		if err != nil {
			sets.opts = append(sets.opts, fx.Error(err))
			return
		}

		sets.opts = append(sets.opts, fxutil.ReplaceAs(key, new(crypto.PrivKey)))
	}

}

// WithHost sets custom Host's data for p2p networking.
func WithHost(hst host.Host) Option {
	return func(sets *settings) {
		sets.opts = append(sets.opts, fxutil.ReplaceAs(hst, new(p2p.HostBase)))
	}
}

// WithCoreClient sets custom client for core process
func WithCoreClient(client core.Client) Option {
	return func(sets *settings) {
		sets.opts = append(sets.opts, fxutil.ReplaceAs(client, new(core.Client)))
	}
}

// WithHeaderConstructFn sets custom func that creates extended header
func WithHeaderConstructFn(construct header.ConstructFn) Option {
	return func(sets *settings) {
		sets.opts = append(sets.opts, fx.Replace(construct))
	}
}

// WithKeyringSigner overrides the default keyring signer constructed
// by the node.
func WithKeyringSigner(signer *apptypes.KeyringSigner) Option {
	return func(sets *settings) {
		sets.opts = append(sets.opts, fx.Replace(signer))
	}
}

// WithBootstrappers sets custom bootstrap peers.
func WithBootstrappers(peers params.Bootstrappers) Option {
	return func(sets *settings) {
		sets.opts = append(sets.opts, fx.Replace(peers))
	}
}

// WithRefreshRoutingTablePeriod sets custom refresh period for dht.
// Currently, it is used to speed up tests.
func WithRefreshRoutingTablePeriod(interval time.Duration) Option {
	return func(sets *settings) {
		sets.cfg.P2P.RoutingTableRefreshPeriod = interval
	}
}
