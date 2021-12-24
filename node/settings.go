package node

import (
	"encoding/hex"

	"github.com/libp2p/go-libp2p-core/crypto"

	"github.com/celestiaorg/celestia-node/node/fxutil"
)

// Option for Node's Config.
type Option func(*Config, *settings) error

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

// settings store all the non Config values that can be altered for Node with Options.
type settings struct {
	P2PKey crypto.PrivKey
}

// overrides collects all the custom Modules and Components set to be overridden for the Node.
func (sets *settings) overrides() (opts []fxutil.Option) {
	opts = append(opts, fxutil.OverrideSupply(&sets.P2PKey))
	return
}
