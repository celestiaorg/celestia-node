package node

import (
	"encoding/hex"

	"github.com/libp2p/go-libp2p-core/crypto"
	"github.com/libp2p/go-libp2p-core/host"

	"github.com/celestiaorg/celestia-node/params"

	"github.com/celestiaorg/celestia-node/core"
	"github.com/celestiaorg/celestia-node/node/fxutil"
	"github.com/celestiaorg/celestia-node/node/p2p"
)

// Option for Node's Config.
type Option func(*Config, *settings) error

// WithNetwork specifies the Network to which the Node should connect to.
// WARNING: Use this option with caution and never run the Node with different networks over the same persisted Store.
func WithNetwork(net params.Network) Option {
	return func(cfg *Config, sets *settings) error {
		sets.Network = net
		return nil
	}
}

// WithP2PKey sets custom Ed25519 private key for p2p networking.
func WithP2PKey(key crypto.PrivKey) Option {
	return func(cfg *Config, sets *settings) (_ error) {
		sets.P2PKey = key
		return
	}
}

// WithP2PKeyStr sets custom hex encoded Ed25519 private key for p2p networking.
func WithP2PKeyStr(key string) Option {
	return func(cfg *Config, sets *settings) (_ error) {
		decKey, err := hex.DecodeString(key)
		if err != nil {
			return err
		}

		key, err := crypto.UnmarshalEd25519PrivateKey(decKey)
		if err != nil {
			return err
		}

		sets.P2PKey = key
		return
	}

}

// WithHost sets custom Host's data for p2p networking.
func WithHost(host host.Host) Option {
	return func(cfg *Config, sets *settings) (_ error) {
		sets.Host = host
		return
	}
}

// WithCoreClient sets custom client for core process
func WithCoreClient(client core.Client) Option {
	return func(cfg *Config, sets *settings) (_ error) {
		sets.CoreClient = client
		return
	}
}

// settings store all the non Config values that can be altered for Node with Options.
type settings struct {
	Network    params.Network
	P2PKey     crypto.PrivKey
	Host       p2p.HostBase
	CoreClient core.Client
}

// overrides collects all the custom Modules and Components set to be overridden for the Node.
// TODO(@Bidon15): Pass settings instead of overrides func. Issue #300
func (sets *settings) overrides() fxutil.Option {
	return fxutil.OverrideSupply(
		&sets.Network,
		&sets.P2PKey,
		&sets.Host,
		&sets.CoreClient,
	)
}
