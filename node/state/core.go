package state

import (
	"fmt"
	"os"

	lens "github.com/strangelove-ventures/lens/client"

	"github.com/celestiaorg/celestia-node/node"
	"github.com/celestiaorg/celestia-node/node/core"
	"github.com/celestiaorg/celestia-node/node/fxutil"
	"github.com/celestiaorg/celestia-node/service/state"
)

// StateOverCoreComponents collects all components necessary for running StateAccess
// over celestia-core.
func StateOverCoreComponents(cfg node.Config, store node.Store) fxutil.Option {
	// only provide components if trusted peer exists
	trustedPeerExists := cfg.Services.TrustedPeer != ""
	return fxutil.Options(
		fxutil.ProvideIf(trustedPeerExists, func() (*lens.ChainClient, error) {
			return ChainClient(cfg.Core, store)
		}),
		fxutil.ProvideIf(trustedPeerExists, state.NewCoreAccessor),
	)
}

func ChainClient(coreConfig core.Config, store node.Store) (*lens.ChainClient, error) {
	if !coreConfig.Remote {
		// TODO @renaynay: hand it Client interface https://github.com/strangelove-ventures/lens/pull/94
	}
	conf := DefaultCelestiaChainClientConfig(coreConfig, fmt.Sprintf("%s/keys", store.Path()))
	return lens.NewChainClient(conf, store.Path(), os.Stdin, os.Stdout)
}

func DefaultCelestiaChainClientConfig(coreConfig core.Config, keyDir string) *lens.ChainClientConfig {
	return &lens.ChainClientConfig{
		Key:          "default",    // TODO @renaynay idk about this
		ChainID:      "celestia-1", // TODO @renaynay should be hardcoded somewhere as a var
		RPCAddr:      coreConfig.RemoteConfig.RemoteAddr,
		KeyDirectory: keyDir,
		Timeout:      "20s",
		OutputFormat: "json",
	}
}
