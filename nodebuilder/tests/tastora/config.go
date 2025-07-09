package tastora

// Config represents configuration options for the Tastora testing framework.
type Config struct {
	NumValidators   int
	ChainFullNodes  int // Celestia blockchain full nodes
	FullNodeCount   int // DA network full nodes (celestia-node instances)
	BridgeNodeCount int
	LightNodeCount  int
}

// Option for modifying Tastora's Config.
type Option func(*Config)

// defaultConfig creates the default configuration for Tastora tests.
func defaultConfig() *Config {
	return &Config{
		NumValidators:   1,
		ChainFullNodes:  0, // Usually no blockchain full nodes needed in tests
		FullNodeCount:   1, // DA network full nodes
		BridgeNodeCount: 1,
		LightNodeCount:  1,
	}
}

// WithValidators sets the number of validators for the chain.
func WithValidators(count int) Option {
	return func(c *Config) {
		c.NumValidators = count
	}
}

// WithChainFullNodes sets the number of full nodes in the Celestia blockchain.
// These are blockchain full nodes, not DA nodes.
func WithChainFullNodes(count int) Option {
	return func(c *Config) {
		c.ChainFullNodes = count
	}
}

// WithFullNodes sets the number of full nodes in the DA network.
// These are celestia-node instances, not blockchain nodes.
func WithFullNodes(count int) Option {
	return func(c *Config) {
		c.FullNodeCount = count
	}
}

// WithBridgeNodes sets the number of bridge nodes in the DA network.
func WithBridgeNodes(count int) Option {
	return func(c *Config) {
		c.BridgeNodeCount = count
	}
}

// WithLightNodes sets the number of light nodes in the DA network.
func WithLightNodes(count int) Option {
	return func(c *Config) {
		c.LightNodeCount = count
	}
}

// WithBlockTime sets the block time for the chain (for sync testing).
func WithBlockTime(blockTime int) Option {
	return func(c *Config) {
		// This would be used in chain configuration
		// Implementation depends on how block time is configured in Tastora
	}
}
