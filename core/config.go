package core

import (
	"os"
	"testing"

	"github.com/BurntSushi/toml"

	"github.com/celestiaorg/celestia-core/config"
)

type Config = config.Config

func DefaultConfig() *Config {
	cfg := config.DefaultConfig()
	updateDefaults(cfg)
	return cfg
}

func TestConfig(t *testing.T) *Config {
	cfg := config.ResetTestRoot(t.Name())
	t.Cleanup(func() {
		os.RemoveAll(cfg.RootDir)
	})
	return cfg
}

// LoadConfig config from the file under the given 'path'.
// NOTE: Unfortunately, Core(Tendermint) does not provide us with convenient function to load the Config,
// so we have to do our own. Furthermore, we have to make own SaveConfig, as the one the Core provides has
// incompatibility issues with our toml parser version.
func LoadConfig(path string) (*Config, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var cfg config.Config
	_, err = toml.NewDecoder(f).Decode(&cfg)
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