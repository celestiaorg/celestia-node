package state

import (
	"fmt"
	"os"

	lens "github.com/strangelove-ventures/lens/client"

	"github.com/celestiaorg/celestia-node/node/core"
)

// ChainClient constructs a new `lens.ChainClient` that can be used
// to access state-related information over the active celestia-core
// connection.
func ChainClient(cfg core.Config, storePath string) (*lens.ChainClient, error) {
	// TODO @renaynay: eventually handle for bridge nodes so that CC constructor can
	//  take a Client interface https://github.com/strangelove-ventures/lens/pull/94
	conf := DefaultCelestiaChainClientConfig(cfg, fmt.Sprintf("%s/keys", storePath))
	return lens.NewChainClient(conf, storePath, os.Stdin, os.Stdout)
}

func DefaultCelestiaChainClientConfig(cfg core.Config, keyDir string) *lens.ChainClientConfig {
	return &lens.ChainClientConfig{
		Key:            "default",    // TODO @renaynay idk about this
		ChainID:        "celestia-1", // TODO @renaynay should be hardcoded somewhere as a var
		RPCAddr:        fmt.Sprintf("%s://%s", cfg.RemoteConfig.Protocol, cfg.RemoteConfig.RemoteAddr),
		KeyDirectory:   keyDir,
		Timeout:        "20s",
		OutputFormat:   "json",
		KeyringBackend: "test", // TODO @renaynay: fix this
	}
}
