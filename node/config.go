package node

import (
	"github.com/celestiaorg/celestia-node/node/p2p"
	"github.com/celestiaorg/celestia-node/rpc"
)

// Config is main configuration structure for a Node.
// It combines configuration units for all Node subsystems.
type Config struct {
	P2P *p2p.Config
	RPC *rpc.Config
}

// DefaultConfig provides a default Node Config.
func DefaultConfig() *Config {
	return &Config{
		P2P: p2p.DefaultConfig(),
	}
}
