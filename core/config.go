package core

import (
	"os"

	"github.com/BurntSushi/toml"

	"github.com/celestiaorg/celestia-core/config"
)

// Config is an alias for Core config.
// For the simplicity of future config changes, the alias should be used throughout celestia-node codebase/repo
type Config = config.Config

// DefaultConfig returns a modified default config of Core node.
func DefaultConfig() *Config {
	cfg := config.DefaultConfig()
	updateDefaults(cfg)
	return cfg
}

// LoadConfig loads the config from the file under the given 'path'.
// NOTE: Unfortunately, Core(Tendermint) does not provide us with convenient function to load the Config,
// so we have to do our own. Furthermore, we have to make own SaveConfig, as the one the Core provides has
// incompatibility issues with our toml parser version.
func LoadConfig(path string) (*Config, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var cfg Config
	_, err = toml.DecodeReader(f, &cfg)
	return &cfg, err
}

// SaveConfig saves config in a file under the given 'path'.
func SaveConfig(path string, cfg *Config) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	return toml.NewEncoder(f).Encode(cfg)
}

// change default paths to remove unneeded 'config' sub dir
func updateDefaults(cfg *Config) {
	cfg.Genesis = "genesis.json"
	cfg.PrivValidatorKey = "priv_validator_key.json"
	cfg.PrivValidatorState = "priv_validator_state.json"
	cfg.NodeKey = "p2p_key.json"
	cfg.P2P.AddrBook = "addr_book.json"
}
