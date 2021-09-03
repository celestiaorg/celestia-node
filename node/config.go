package node

import "github.com/celestiaorg/celestia-node/node/p2p"

// Config is main configuration structure for a Node.
// It combines configuration units for all Node subsystems.
type Config struct {
	P2P *p2p.Config
}

// EmptyConfig creates an empty Node Config.
// Its main purpose is to fill out pointers not to run into nil pointer dereference.
func EmptyConfig() *Config {
	return &Config{
		P2P: &p2p.Config{},
	}
}
