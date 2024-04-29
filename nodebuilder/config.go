package nodebuilder

import (
	"fmt"
	"io"
	"os"

	"github.com/BurntSushi/toml"
	"github.com/gofrs/flock"
	"github.com/imdario/mergo"

	"github.com/celestiaorg/celestia-node/nodebuilder/core"
	"github.com/celestiaorg/celestia-node/nodebuilder/das"
	"github.com/celestiaorg/celestia-node/nodebuilder/gateway"
	"github.com/celestiaorg/celestia-node/nodebuilder/header"
	"github.com/celestiaorg/celestia-node/nodebuilder/node"
	"github.com/celestiaorg/celestia-node/nodebuilder/p2p"
	"github.com/celestiaorg/celestia-node/nodebuilder/rpc"
	"github.com/celestiaorg/celestia-node/nodebuilder/share"
	"github.com/celestiaorg/celestia-node/nodebuilder/state"
)

// ConfigLoader defines a function that loads a config from any source.
type ConfigLoader func() (*Config, error)

// Config is main configuration structure for a Node.
// It combines configuration units for all Node subsystems.
type Config struct {
	Node    node.Config
	Core    core.Config
	State   state.Config
	P2P     p2p.Config
	RPC     rpc.Config
	Gateway gateway.Config
	Share   share.Config
	Header  header.Config
	DASer   das.Config `toml:",omitempty"`
}

// DefaultConfig provides a default Config for a given Node Type 'tp'.
// NOTE: Currently, configs are identical, but this will change.
func DefaultConfig(tp node.Type) *Config {
	commonConfig := &Config{
		Node:    node.DefaultConfig(tp),
		Core:    core.DefaultConfig(),
		State:   state.DefaultConfig(),
		P2P:     p2p.DefaultConfig(tp),
		RPC:     rpc.DefaultConfig(),
		Gateway: gateway.DefaultConfig(),
		Share:   share.DefaultConfig(tp),
		Header:  header.DefaultConfig(tp),
	}

	switch tp {
	case node.Bridge:
		return commonConfig
	case node.Light, node.Full:
		commonConfig.DASer = das.DefaultConfig(tp)
		return commonConfig
	default:
		panic("node: invalid node type")
	}
}

// SaveConfig saves Config 'cfg' under the given 'path'.
func SaveConfig(path string, cfg *Config) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	return cfg.Encode(f)
}

// LoadConfig loads Config from the given 'path'.
func LoadConfig(path string) (*Config, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var cfg Config
	return &cfg, cfg.Decode(f)
}

// RemoveConfig removes the Config from the given store path.
func RemoveConfig(path string) (err error) {
	path, err = storePath(path)
	if err != nil {
		return
	}

	flk := flock.New(lockPath(path))
	ok, err := flk.TryLock()
	if err != nil {
		return fmt.Errorf("locking file: %w", err)
	}
	if !ok {
		return ErrOpened
	}
	defer flk.Unlock() //nolint:errcheck

	return removeConfig(configPath(path))
}

// removeConfig removes Config from the given 'path'.
func removeConfig(path string) error {
	return os.Remove(path)
}

// UpdateConfig loads the node's config and applies new values
// from the default config of the given node type, saving the
// newly updated config into the node's config path.
func UpdateConfig(tp node.Type, path string) (err error) {
	path, err = storePath(path)
	if err != nil {
		return err
	}

	flk := flock.New(lockPath(path))
	ok, err := flk.TryLock()
	if err != nil {
		return fmt.Errorf("locking file: %w", err)
	}
	if !ok {
		return ErrOpened
	}
	defer flk.Unlock() //nolint:errcheck

	newCfg := DefaultConfig(tp)

	cfgPath := configPath(path)
	cfg, err := LoadConfig(cfgPath)
	if err != nil {
		return err
	}

	cfg, err = updateConfig(cfg, newCfg)
	if err != nil {
		return err
	}

	// save the updated config
	err = removeConfig(cfgPath)
	if err != nil {
		return err
	}
	return SaveConfig(cfgPath, cfg)
}

// updateConfig merges new values from the new config into the old
// config, returning the updated old config.
func updateConfig(oldCfg, newCfg *Config) (*Config, error) {
	err := mergo.Merge(oldCfg, newCfg, mergo.WithOverrideEmptySlice)
	return oldCfg, err
}

// TODO(@Wondertan): We should have a description for each field written into w,
// 	so users can instantly understand purpose of each field. Ideally, we should have a utility
// program to parse comments 	from actual sources(*.go files) and generate docs from comments.

// Hint: use 'ast' package.
// Encode encodes a given Config into w.
func (cfg *Config) Encode(w io.Writer) error {
	return toml.NewEncoder(w).Encode(cfg)
}

// Decode decodes a Config from a given reader r.
func (cfg *Config) Decode(r io.Reader) error {
	_, err := toml.NewDecoder(r).Decode(cfg)
	return err
}
