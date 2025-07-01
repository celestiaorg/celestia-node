package tastora

// Config represents configuration options for the Tastora testing framework.
type Config struct {
	NumValidators   int
	NumFullNodes    int
	FullNodeCount   int
	BridgeNodeCount int
	LightNodeCount  int
}

// Option for modifying Tastora's Config.
type Option func(*Config)

// defaultConfig creates the default configuration for Tastora tests.
func defaultConfig() *Config {
	return &Config{
		NumValidators:   1,
		NumFullNodes:    0,
		FullNodeCount:   1,
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

// WithFullNodes sets the number of full nodes in the DA network.
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
