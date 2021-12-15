package node

import (
	"encoding/hex"

	"github.com/libp2p/go-libp2p-core/crypto"

	"github.com/celestiaorg/celestia-node/node/fxutil"
)

// Option for Node's Config.
type Option func(*Config) error

// WithRemoteCore configures Node to start with remote Core.
func WithRemoteCore(protocol string, address string) Option {
	return func(cfg *Config) (_ error) {
		cfg.Core.Remote = true
		cfg.Core.RemoteConfig.Protocol = protocol
		cfg.Core.RemoteConfig.RemoteAddr = address
		return
	}
}

// WithTrustedHash sets TrustedHash to the Config.
func WithTrustedHash(hash string) Option {
	return func(cfg *Config) (_ error) {
		cfg.Services.TrustedHash = hash
		return
	}
}

// WithTrustedPeer sets TrustedPeer to the Config.
func WithTrustedPeer(addr string) Option {
	return func(cfg *Config) (_ error) {
		cfg.Services.TrustedPeer = addr
		return
	}
}

// WithConfig sets the entire custom config.
func WithConfig(custom *Config) Option {
	return func(cfg *Config) (_ error) {
		*cfg = *custom
		return
	}
}

// WithP2PKey sets custom Ed25519 private key for p2p networking.
func WithP2PKey(key crypto.PrivKey) Option {
	return func(cfg *Config) (_ error) {
		cfg.overrides = append(cfg.overrides, fxutil.OverrideSupply(&key))
		return
	}
}

// WithP2PKeyStr sets custom hex encoded Ed25519 private key for p2p networking.
func WithP2PKeyStr(key string) Option {
	return func(cfg *Config) (_ error) {
		decKey, err := hex.DecodeString(key)
		if err != nil {
			return err
		}

		key, err := crypto.UnmarshalEd25519PrivateKey(decKey)
		if err != nil {
			return err
		}

		cfg.overrides = append(cfg.overrides, fxutil.OverrideSupply(&key))
		return
	}
}

func WithMutualPeers(addrs []string) Option {
	return func(cfg *Config) {
		cfg.P2P.MutualPeers = addrs
	}
}
