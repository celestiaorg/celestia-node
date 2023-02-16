package nodebuilder

import (
	"io"
	"os"

	"github.com/BurntSushi/toml"

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
		Core:    core.DefaultConfig(),
		State:   state.DefaultConfig(),
		P2P:     p2p.DefaultConfig(tp),
		RPC:     rpc.DefaultConfig(),
		Gateway: gateway.DefaultConfig(),
		Share:   share.DefaultConfig(),
		Header:  header.DefaultConfig(tp),
	}

	switch tp {
	case node.Bridge:
		return commonConfig
	case node.Light, node.Full:
		commonConfig.DASer = das.DefaultConfig()
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
