package p2p

import (
	"encoding/hex"

	hst "github.com/libp2p/go-libp2p/core/host"

	"github.com/libp2p/go-libp2p/core/crypto"
	"go.uber.org/fx"

	"github.com/celestiaorg/celestia-node/libs/fxutil"
)

// WithP2PKey sets custom Ed25519 private key for p2p networking.
func WithP2PKey(key crypto.PrivKey) fx.Option {
	return fxutil.ReplaceAs(key, new(crypto.PrivKey))
}

// WithP2PKeyStr sets custom hex encoded Ed25519 private key for p2p networking.
func WithP2PKeyStr(key string) fx.Option {
	decKey, err := hex.DecodeString(key)
	if err != nil {
		return fx.Error(err)
	}

	privKey, err := crypto.UnmarshalEd25519PrivateKey(decKey)
	if err != nil {
		return fx.Error(err)
	}

	return fxutil.ReplaceAs(privKey, new(crypto.PrivKey))
}

// WithHost sets custom Host's data for p2p networking.
func WithHost(hst hst.Host) fx.Option {
	return fxutil.ReplaceAs(hst, new(HostBase))
}
